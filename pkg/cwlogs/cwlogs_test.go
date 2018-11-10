package cwlogs

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type testApi struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
	cb func(ctx aws.Context, input *cloudwatchlogs.FilterLogEventsInput, pager func(*cloudwatchlogs.FilterLogEventsOutput, bool) bool, opts ...request.Option) error
}

func (a *testApi) FilterLogEventsPagesWithContext(ctx aws.Context, input *cloudwatchlogs.FilterLogEventsInput, pager func(*cloudwatchlogs.FilterLogEventsOutput, bool) bool, opts ...request.Option) error {
	return a.cb(ctx, input, pager, opts...)
}

func TestStream(t *testing.T) {
	api := &testApi{}

	commandId := "83b98484-4a9b-4470-ab17-e4a646e2a72e"

	ctx, cancel := context.WithCancel(context.Background())
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        aws.String("/aws/ssm/AWS-RunShellScript"),
		LogStreamNamePrefix: &commandId,
	}

	stream := &CwStream{Input: input, Sleep: time.Microsecond}

	// this is grotesque
	api.cb = func(ctx aws.Context, pageInput *cloudwatchlogs.FilterLogEventsInput, pager func(*cloudwatchlogs.FilterLogEventsOutput, bool) bool, opts ...request.Option) error {
		assert.EqualValues(t, pageInput, input)
		pager(&cloudwatchlogs.FilterLogEventsOutput{
			Events: []*cloudwatchlogs.FilteredLogEvent{
				{
					LogStreamName: aws.String("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-abc123/aws-runShellScript/stdout"),
					EventId:       aws.String("ev10"),
					Timestamp:     aws.Int64(50),
				},
				{
					LogStreamName: aws.String("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-abc123/aws-runShellScript/stdout"),
					EventId:       aws.String("ev11"),
					Timestamp:     aws.Int64(55),
				},
				{
					LogStreamName: aws.String("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-def456/aws-runShellScript/stdout"),
					EventId:       aws.String("ev20"),
					Timestamp:     aws.Int64(60),
				},
				{
					LogStreamName: aws.String("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-def456/aws-runShellScript/stdout"),
					EventId:       aws.String("ev21"),
					Timestamp:     aws.Int64(65),
				},
			},
		}, true)

		api.cb = func(ctx aws.Context, pageInput *cloudwatchlogs.FilterLogEventsInput, pager func(*cloudwatchlogs.FilterLogEventsOutput, bool) bool, opts ...request.Option) error {
			assert.EqualValues(t, 56, *pageInput.StartTime)

			stream.IgnorePrefix("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-abc123")

			pager(&cloudwatchlogs.FilterLogEventsOutput{
				Events: []*cloudwatchlogs.FilteredLogEvent{
					{
						LogStreamName: aws.String("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-abc123/aws-runShellScript/stdout"),
						EventId:       aws.String("ev12"),
						Timestamp:     aws.Int64(58),
					},
					{
						LogStreamName: aws.String("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-def456/aws-runShellScript/stdout"),
						EventId:       aws.String("ev22"),
						Timestamp:     aws.Int64(67),
					},
					{
						LogStreamName: aws.String("83b98484-4a9b-4470-ab17-e4a646e2a72e/i-def456/aws-runShellScript/stdout"),
						EventId:       aws.String("ev23"),
						Timestamp:     aws.Int64(69),
					},
				},
			}, true)

			api.cb = func(ctx aws.Context, pageInput *cloudwatchlogs.FilterLogEventsInput, pager func(*cloudwatchlogs.FilterLogEventsOutput, bool) bool, opts ...request.Option) error {
				assert.EqualValues(t, 70, *pageInput.StartTime)
				cancel()
				return nil
			}

			return nil
		}
		return nil
	}

	ch := make(chan *cloudwatchlogs.FilteredLogEvent)
	go stream.Stream(ctx, api, ch)

	idx := 0
	expectedEventIds := []string{"ev10", "ev11", "ev20", "ev21", "ev22", "ev23"}
	for ev := range ch {
		assert.EqualValues(t, expectedEventIds[idx], *ev.EventId)
		idx++
	}
}
