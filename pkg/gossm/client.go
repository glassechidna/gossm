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
	"log"
	"math"
	"strings"
	"time"
)

type Client struct {
	ssmApi  ssmiface.SSMAPI
	ec2Api  ec2iface.EC2API
	logsApi cloudwatchlogsiface.CloudWatchLogsAPI
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

func (c *Client) Doit(ctx context.Context, commandInput *ssm.SendCommandInput) (*DoitResponse, error) {
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

	go c.poll(ctx, resp, response)
	return response, nil
}

func (c *Client) waitDone(ctx context.Context, commandId *string, resp *DoitResponse) {
	printedInstanceIds := []string{}
	instanceIds := resp.InstanceIds
	expectedResponseCount := len(instanceIds.InstanceIds) - len(instanceIds.FaultyInstanceIds) - len(instanceIds.WrongPlatformInstanceIds)

	for {
		time.Sleep(time.Second * 3)

		listInput := &ssm.ListCommandInvocationsInput{CommandId: commandId}
		resp3, err := c.ssmApi.ListCommandInvocations(listInput)

		if err != nil {
			panic(err)
		}

		for _, invocation := range resp3.CommandInvocations {
			instanceId := *invocation.InstanceId
			if stringInSlice(instanceId, printedInstanceIds) ||
				stringInSlice(instanceId, instanceIds.FaultyInstanceIds) ||
				stringInSlice(instanceId, instanceIds.WrongPlatformInstanceIds) {
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

func (c *Client) pollLogs(ctx context.Context, output *ssm.SendCommandOutput, resp *DoitResponse) {
	commandId := output.Command.CommandId
	group := fmt.Sprintf("/aws/ssm/%s", *output.Command.DocumentName)

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &group,
		LogStreamNamePrefix: commandId,
	}

	lastTimestamps := map[string]int64{}
	for _, id := range resp.InstanceIds.InstanceIds {
		lastTimestamps[id] = 0
	}

	seenEvents := map[string]bool{}

	for {
		time.Sleep(time.Second)

		err := c.logsApi.FilterLogEventsPagesWithContext(ctx, input, func(page *cloudwatchlogs.FilterLogEventsOutput, lastPage bool) bool {
			for _, event := range page.Events {
				msg := logEventToSsmMessage(event)

				if _, ok := seenEvents[*event.EventId]; !ok {
					resp.Channel <- msg
					seenEvents[*event.EventId] = true
				}

				lastTimestamps[msg.InstanceId] = *event.Timestamp
			}
			return !lastPage
		})
		if err != nil && err != context.Canceled {
			panic(err)
		}

		var lastTimestamp int64 = math.MaxInt64
		for _, ts := range lastTimestamps {
			if ts < lastTimestamp {
				lastTimestamp = ts
			}
		}
		lastTimestamp++
		input.StartTime = &lastTimestamp

		if ctx.Err() == context.Canceled {
			break
		}
	}
}

func (c *Client) poll(ctx context.Context, output *ssm.SendCommandOutput, resp *DoitResponse) {
	defer close(resp.Channel)

	newctx, cancel := context.WithCancel(ctx)

	go c.pollLogs(newctx, output, resp)
	c.waitDone(ctx, output.Command.CommandId, resp)

	select {
	case <-ctx.Done():
		break
	case <-time.After(5*time.Second):
		cancel()
		break
	}
}
