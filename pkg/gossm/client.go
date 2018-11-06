package gossm

import (
	"context"
	"fmt"
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

type SsmPayloadMessage struct {
	InstanceId  string
	StdoutChunk string
	StderrChunk string
}

type SsmControlMessage struct {
	Invocations Invocations
}

type SsmMessage struct {
	CommandId string
	Error     error
	Control   *SsmControlMessage
	Payload   *SsmPayloadMessage
}

type DoitResponse struct {
	CommandId   string
	InstanceIds *InstanceIds
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

func (c *Client) Doit(ctx context.Context, commandInput *ssm.SendCommandInput) (*DoitResponse, error) {
	resp, err := c.ssmApi.SendCommand(commandInput)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Second * 3)

	commandId := *resp.Command.CommandId

	invocations := Invocations{}
	err = invocations.AddFromSSM(c.ssmApi, commandId)
	if err != nil {
		return nil, err
	}

	instanceIds, err := invocations.InstanceIds(c.ec2Api)
	if err != nil {
		return nil, err
	}

	response := &DoitResponse{
		CommandId:   commandId,
		InstanceIds: instanceIds,
	}

	return response, nil
}

func (c *Client) pollStatusUntilDone(ctx context.Context, commandId string, ch chan Invocations) {
	prev := Invocations{}

	for {
		time.Sleep(time.Second * 3)

		invocations := Invocations{}
		err := invocations.AddFromSSM(c.ssmApi, commandId)
		if err != nil {
			panic(err)
		}

		if len(invocations.CompletedSince(prev)) > 0 {
			ch <- invocations
		}

		if invocations.AllComplete() {
			return
		}
	}
}

func (c *Client) cwLogsInput(commandId string) (*cloudwatchlogs.FilterLogEventsInput, error) {
	invs := Invocations{}
	err := invs.AddFromSSM(c.ssmApi, commandId)
	if err != nil {
		return nil, err
	}

	docName := ""
	for _, inv := range invs {
		docName = *inv.DocumentName
		break
	}

	group := fmt.Sprintf("/aws/ssm/%s", docName)
	return &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &group,
		LogStreamNamePrefix: &commandId,
	}, nil
}

func (c *Client) Poll(ctx context.Context, commandId string, ch chan SsmMessage) error {
	defer close(ch)

	input, err := c.cwLogsInput(commandId)
	if err != nil {
		return err
	}

	cw := cwlogs.New(c.logsApi)
	s := cw.Stream(ctx, input)

	statusCh := make(chan Invocations)
	prevStatus := Invocations{}
	go c.pollStatusUntilDone(ctx, commandId, statusCh)

	done := make(chan bool)

	instanceCompleted := func(id string) {
		// TODO: fix duplication
		s.Ignore(strings.Join([]string{commandId, id, "aws-runShellScript", "stdout"}, "/"))
		s.Ignore(strings.Join([]string{commandId, id, "aws-runShellScript", "stderr"}, "/"))
		s.Ignore(strings.Join([]string{commandId, id, "aws-runPowerShellScript", "stdout"}, "/"))
		s.Ignore(strings.Join([]string{commandId, id, "aws-runPowerShellScript", "stderr"}, "/"))
	}

	for {
		select {
		case event := <-s.Channel:
			ch <- logEventToSsmMessage(event)
		case invocations := <-statusCh:
			ch <- SsmMessage{
				CommandId: commandId,
				Control: &SsmControlMessage{
					Invocations: invocations,
				},
			}
			changed := invocations.CompletedSince(prevStatus)
			prevStatus = invocations
			for id := range changed {
				time.AfterFunc(5*time.Second, func() { instanceCompleted(id) })
			}
			if invocations.AllComplete() {
				fmt.Println("started countdown")
				time.AfterFunc(5*time.Second, func() { done <- true })
			}
		case <-ctx.Done():
			fmt.Println("cancelled")
			return nil
		case <-done:
			return nil
		}
	}
}

func logEventToSsmMessage(event *cloudwatchlogs.FilteredLogEvent) SsmMessage {
	bits := strings.Split(*event.LogStreamName, "/")
	commandId := bits[0]
	instanceId := bits[1]
	isStdout := bits[3] == "stdout"

	msg := SsmMessage{
		CommandId: commandId,
		Payload: &SsmPayloadMessage{
			InstanceId: instanceId,
		},
	}

	if isStdout {
		msg.Payload.StdoutChunk = *event.Message
	} else {
		msg.Payload.StderrChunk = *event.Message
	}

	return msg
}
