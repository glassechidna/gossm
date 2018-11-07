package gossm

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/glassechidna/gossm/pkg/cwlogs"
	"strings"
	"time"
)

type Client struct {
	ssmApi  ssmiface.SSMAPI
	logsApi cloudwatchlogsiface.CloudWatchLogsAPI
	history *History
}

type SsmPayloadMessage struct {
	InstanceId  string
	StdoutChunk string
	StderrChunk string
}

type SsmControlMessage struct {
	Status Status
}

type SsmMessage struct {
	CommandId string
	Error     error
	Control   *SsmControlMessage
	Payload   *SsmPayloadMessage
}

func New(sess *session.Session, history *History) *Client {
	return &Client{
		ssmApi:  ssm.New(sess),
		logsApi: cloudwatchlogs.New(sess),
		history: history,
	}
}

type Status struct {
	Command     *ssm.Command
	Invocations Invocations
}

func (c *Client) Doit(ctx context.Context, commandInput *ssm.SendCommandInput) (*Status, error) {
	resp, err := c.ssmApi.SendCommand(commandInput)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Second * 3)

	invocations := Invocations{}
	err = invocations.AddFromSSM(c.ssmApi, *resp.Command.CommandId)
	if err != nil {
		return nil, err
	}

	err = c.history.PutCommand(resp.Command, invocations)
	if err != nil {
		return nil, err
	}

	response := &Status{
		Command:     resp.Command,
		Invocations: invocations,
	}

	return response, nil
}

func (c *Client) pollStatusUntilDone(ctx context.Context, commandId string, ch chan Status) {
	prev := Invocations{}

	for {
		time.Sleep(time.Second * 3)

		resp, err := c.ssmApi.ListCommands(&ssm.ListCommandsInput{CommandId: &commandId})
		if err != nil {
			panic(err)
		}

		invocations := Invocations{}
		err = invocations.AddFromSSM(c.ssmApi, commandId)
		if err != nil {
			panic(err)
		}

		if len(invocations.CompletedSince(prev)) > 0 {
			ch <- Status{Command: resp.Commands[0], Invocations: invocations}
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

	statusCh := make(chan Status)
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
			msg := logEventToSsmMessage(event)
			err = c.history.AppendPayload(msg)
			ch <- msg
		case status := <-statusCh:
			ch <- SsmMessage{
				CommandId: commandId,
				Control:   &SsmControlMessage{Status: status},
			}
			err = c.history.PutCommand(status.Command, status.Invocations)
			if err != nil {
				return err
			}
			changed := status.Invocations.CompletedSince(prevStatus)
			prevStatus = status.Invocations
			for id := range changed {
				time.AfterFunc(5*time.Second, func() { instanceCompleted(id) })
			}
			if status.Invocations.AllComplete() {
				time.AfterFunc(5*time.Second, func() { done <- true })
			}
		case <-ctx.Done():
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
