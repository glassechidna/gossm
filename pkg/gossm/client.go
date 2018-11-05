package gossm

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/glassechidna/gossm/pkg/cwlogs"
	"strings"
	"time"
)

type Client struct {
	ssmApi  ssmiface.SSMAPI
	ec2Api  ec2iface.EC2API
	logsApi cloudwatchlogsiface.CloudWatchLogsAPI
}

type CommandInstanceIdsOutput struct {
	DocumentName             string
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
	CommandId   string
	InstanceIds *CommandInstanceIdsOutput
}

func New(sess *session.Session) *Client {
	return &Client{
		ssmApi:  ssm.New(sess),
		ec2Api:  ec2.New(sess),
		logsApi: cloudwatchlogs.New(sess),
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

func (c *Client) commandInstanceIds(commandId string) (*CommandInstanceIdsOutput, error) {
	invocations := []*ssm.CommandInvocation{}

	listInput := &ssm.ListCommandInvocationsInput{CommandId: &commandId}
	err := c.ssmApi.ListCommandInvocationsPages(listInput, func(page *ssm.ListCommandInvocationsOutput, lastPage bool) bool {
		invocations = append(invocations, page.CommandInvocations...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}

	invocationMap := map[string]*ssm.CommandInvocation{}

	instanceIds := []string{}
	for _, invocation := range invocations {
		id := *invocation.InstanceId
		instanceIds = append(instanceIds, id)
		invocationMap[id] = invocation
	}

	instanceDescs, err := c.ec2Api.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})
	if err != nil {
		return nil, err
	}

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

	return &CommandInstanceIdsOutput{
		DocumentName:             *invocations[0].DocumentName,
		InstanceIds:              instanceIds,
		FaultyInstanceIds:        faulty,
		WrongPlatformInstanceIds: wrongInstanceIds,
	}, nil
}

func (c *Client) Doit(ctx context.Context, commandInput *ssm.SendCommandInput) (*DoitResponse, error) {
	resp, err := c.ssmApi.SendCommand(commandInput)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Second * 3)

	commandId := resp.Command.CommandId
	instanceIds, err := c.commandInstanceIds(*commandId)
	if err != nil {
		return nil, err
	}

	response := &DoitResponse{
		CommandId:   *commandId,
		InstanceIds: instanceIds,
	}

	return response, nil
}

func (c *Client) waitDone(ctx context.Context, commandId string, outp *CommandInstanceIdsOutput) {
	printedInstanceIds := []string{}
	expectedResponseCount := len(outp.InstanceIds) - len(outp.FaultyInstanceIds) - len(outp.WrongPlatformInstanceIds)

	for {
		time.Sleep(time.Second * 3)

		listInput := &ssm.ListCommandInvocationsInput{CommandId: &commandId}
		resp3, err := c.ssmApi.ListCommandInvocations(listInput)

		if err != nil {
			panic(err)
		}

		for _, invocation := range resp3.CommandInvocations {
			instanceId := *invocation.InstanceId
			if stringInSlice(instanceId, printedInstanceIds) ||
				stringInSlice(instanceId, outp.FaultyInstanceIds) ||
				stringInSlice(instanceId, outp.WrongPlatformInstanceIds) {
				continue
			}

			if *invocation.Status != ssm.CommandInvocationStatusInProgress {
				printedInstanceIds = append(printedInstanceIds, instanceId)
			}
		}

		if len(printedInstanceIds) == expectedResponseCount || ctx.Err() == context.Canceled {
			return
		}
	}
}

func logEventToSsmMessage(event *cloudwatchlogs.FilteredLogEvent) SsmMessage {
	bits := strings.Split(*event.LogStreamName, "/")
	commandId := bits[0]
	instanceId := bits[1]
	isStdout := bits[3] == "stdout"

	msg := SsmMessage{
		CommandId:  commandId,
		InstanceId: instanceId,
	}

	if isStdout {
		msg.StdoutChunk = *event.Message
	} else {
		msg.StderrChunk = *event.Message
	}

	return msg
}

func (c *Client) pollLogs(ctx context.Context, commandId string, outp *CommandInstanceIdsOutput, ch chan SsmMessage) {
	group := fmt.Sprintf("/aws/ssm/%s", outp.DocumentName)

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &group,
		LogStreamNamePrefix: &commandId,
	}

	cw := cwlogs.New(c.logsApi)
	s := cw.Stream(ctx, input)

	for event := range s.Channel {
		ch <- logEventToSsmMessage(event)
	}
}

func (c *Client) Poll(ctx context.Context, commandId string, ch chan SsmMessage) error {
	defer close(ch)

	newctx, cancel := context.WithCancel(ctx)

	ids, err := c.commandInstanceIds(commandId)
	if err != nil {
		return err
	}

	go c.pollLogs(newctx, commandId, ids, ch)
	c.waitDone(ctx, commandId, ids)

	select {
	case <-ctx.Done():
		break
	case <-time.After(5 * time.Second):
		cancel()
		break
	}

	return nil
}
