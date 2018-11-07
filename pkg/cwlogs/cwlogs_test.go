package cwlogs

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"testing"
)

type testApi struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
}

func (a *testApi) FilterLogEventsPagesWithContext(aws.Context, *cloudwatchlogs.FilterLogEventsInput, func(*cloudwatchlogs.FilterLogEventsOutput, bool) bool, ...request.Option) error {
	panic("hiya")
	return nil
}

func TestStream(t *testing.T) {
	api := &testApi{}
	c := New(api)

	commandId := "83b98484-4a9b-4470-ab17-e4a646e2a72e"

	ctx, cancel := context.WithCancel(context.Background())
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String("/aws/ssm/AWS-RunShellScript"),
		LogStreamNamePrefix: &commandId,
	}

	stream := c.Stream(ctx, input)

	for msg := range stream.Channel {
		msg.
	}

	cancel()
}