package gossm

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"log"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	ssmApi ssmiface.SSMAPI
	ec2Api ec2iface.EC2API
	s3Api  s3iface.S3API
}

type CommandInstanceIdsOutput struct {
	InstanceIds              []string
	FaultyInstanceIds        []string
	WrongPlatformInstanceIds []string
}

type SsmMessage struct {
	CommandId    string
	InstanceId   string
	StdoutChunk  string
	StderrChunk  string
	Error        error
	InstanceDone bool
}

type DoitResponse struct {
	Channel     chan SsmMessage
	CommandId   string
	InstanceIds CommandInstanceIdsOutput
}

func New(sess *session.Session) *Client {
	return &Client{
		ssmApi: ssm.New(sess),
		ec2Api: ec2.New(sess),
		s3Api:  s3.New(sess),
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func (c *Client) getFromS3Url(urlString string) (*string, error) {
	s3url, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	urlPath := s3url.Path
	parts := strings.SplitN(urlPath, "/", 3)
	bucket := parts[1]
	key := parts[2]

	buff := &aws.WriteAtBuffer{}
	s3dl := &s3manager.Downloader{
		S3:          c.s3Api,
		PartSize:    s3manager.DefaultDownloadPartSize,
		Concurrency: s3manager.DefaultDownloadConcurrency,
	}

	_, err = s3dl.Download(buff, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == s3.ErrCodeNoSuchKey {
				return nil, nil
			}
		}
		return nil, err
	}

	str := string(buff.Bytes())
	return &str, nil
}

func (c *Client) commandInstanceIds(commandId string) CommandInstanceIdsOutput {
	listInput := &ssm.ListCommandInvocationsInput{CommandId: &commandId}

	resp3, err := c.ssmApi.ListCommandInvocations(listInput)
	if err != nil {
		log.Panicf(err.Error())
	}

	invocationMap := map[string]*ssm.CommandInvocation{}

	instanceIds := []string{}
	for _, invocation := range resp3.CommandInvocations {
		id := *invocation.InstanceId
		instanceIds = append(instanceIds, id)
		invocationMap[id] = invocation
	}

	instanceDescs, _ := c.ec2Api.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})

	liveInstanceIds := []string{}
	wrongInstanceIds := []string{}

	for _, reservation := range instanceDescs.Reservations {
		for _, instance := range reservation.Instances {
			invocation := invocationMap[*instance.InstanceId]
			docName := *invocation.DocumentName

			if instance.Platform != nil && *instance.Platform == "windows" && docName != "AWS-RunPowerShellScript" {
				wrongInstanceIds = append(wrongInstanceIds, *instance.InstanceId)
				continue
			} else if instance.Platform == nil /* linux */ && docName != "AWS-RunShellScript" {
				wrongInstanceIds = append(wrongInstanceIds, *instance.InstanceId)
				continue
			}

			if *instance.State.Name == "running" {
				liveInstanceIds = append(liveInstanceIds, *instance.InstanceId)
			}
		}
	}

	faulty := []string{}

	for _, instanceId := range instanceIds {
		if !stringInSlice(instanceId, liveInstanceIds) && !stringInSlice(instanceId, wrongInstanceIds) {
			faulty = append(faulty, instanceId)
		}
	}

	return CommandInstanceIdsOutput{
		InstanceIds:              instanceIds,
		FaultyInstanceIds:        faulty,
		WrongPlatformInstanceIds: wrongInstanceIds,
	}
}

func (c *Client) Doit(commandInput *ssm.SendCommandInput) (*DoitResponse, error) {
	resp, err := c.ssmApi.SendCommand(commandInput)

	if err != nil {
		return nil, err
	}

	time.Sleep(time.Second * 3)

	commandId := resp.Command.CommandId
	instanceIds := c.commandInstanceIds(*commandId)

	response := &DoitResponse{
		Channel:     make(chan SsmMessage),
		CommandId:   *commandId,
		InstanceIds: instanceIds,
	}

	go c.poll(commandId, response)
	return response, nil
}

func (c *Client) poll(commandId *string, resp *DoitResponse) {
	printedInstanceIds := []string{}
	instanceIds := resp.InstanceIds
	expectedResponseCount := len(instanceIds.InstanceIds) - len(instanceIds.FaultyInstanceIds) - len(instanceIds.WrongPlatformInstanceIds)

	for {
		quit := func(err error) {
			resp.Channel <- SsmMessage{Error: err}
			close(resp.Channel)
		}

		time.Sleep(time.Second * 3)

		listInput := &ssm.ListCommandInvocationsInput{CommandId: commandId}
		resp3, err := c.ssmApi.ListCommandInvocations(listInput)

		if err != nil {
			quit(err)
			return
		}

		for _, invocation := range resp3.CommandInvocations {
			instanceId := *invocation.InstanceId
			if stringInSlice(instanceId, printedInstanceIds) ||
				stringInSlice(instanceId, instanceIds.FaultyInstanceIds) ||
				stringInSlice(instanceId, instanceIds.WrongPlatformInstanceIds) {
				continue
			}

			if *invocation.Status != ssm.CommandInvocationStatusInProgress {
				time.Sleep(time.Second * 1)

				msg := SsmMessage{
					CommandId:  *commandId,
					InstanceId: instanceId,
				}

				stdout, err := c.getFromS3Url(*invocation.StandardOutputUrl)
				if err != nil {
					quit(err)
					return
				}
				if stdout != nil {
					msg.StdoutChunk = *stdout
				}

				stderr, err := c.getFromS3Url(*invocation.StandardErrorUrl)
				if err != nil {
					quit(err)
					return
				}
				if stderr != nil {
					msg.StderrChunk = *stderr
				}

				if len(msg.StdoutChunk) > 0 || len(msg.StderrChunk) > 0 {
					msg.InstanceDone = true
					resp.Channel <- msg
					printedInstanceIds = append(printedInstanceIds, instanceId)
				}
			}
		}

		if len(printedInstanceIds) == expectedResponseCount {
			close(resp.Channel)
			return
		}
	}

}
