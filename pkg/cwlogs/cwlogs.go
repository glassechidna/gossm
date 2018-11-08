package cwlogs

import (
	"context"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"math"
	"sync"
	"time"
)

type CwLogs struct {
	api cloudwatchlogsiface.CloudWatchLogsAPI
}

func New(api cloudwatchlogsiface.CloudWatchLogsAPI) *CwLogs {
	return &CwLogs{api: api}
}

func (c *CwLogs) Stream(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput) *cwStream {
	lastTimestamps := map[string]int64{}
	seenEvents := map[string]bool{}

	cs := &cwStream{
		Channel: make(chan *cloudwatchlogs.FilteredLogEvent),
		Sleep:   time.Second,
		mut:     &sync.Mutex{},
	}

	go func() {
		for {
			time.Sleep(cs.Sleep)

			err := c.api.FilterLogEventsPagesWithContext(ctx, input, func(page *cloudwatchlogs.FilterLogEventsOutput, lastPage bool) bool {
				for _, event := range page.Events {
					if stringInSlice(*event.LogStreamName, cs.ignored) {
						continue
					}

					if _, ok := seenEvents[*event.EventId]; !ok {
						cs.Channel <- event
						seenEvents[*event.EventId] = true
					}

					lastTimestamps[*event.LogStreamName] = *event.Timestamp
				}
				return !lastPage
			})
			if err != nil && err != context.Canceled {
				panic(err)
			}

			var lastTimestamp int64 = math.MaxInt64

			cs.mut.Lock()
			for logStreamName, ts := range lastTimestamps {
				if ts < lastTimestamp && !stringInSlice(logStreamName, cs.ignored) {
					lastTimestamp = ts
				}
			}

			cs.mut.Unlock()
			lastTimestamp++
			input.StartTime = &lastTimestamp

			if ctx.Err() == context.Canceled {
				close(cs.Channel)
				break
			}
		}
	}()

	return cs
}

type cwStream struct {
	Channel chan *cloudwatchlogs.FilteredLogEvent
	Sleep   time.Duration
	ignored []string
	mut     *sync.Mutex
}

func (cs *cwStream) Ignore(logStreamName string) {
	cs.mut.Lock()
	defer cs.mut.Unlock()
	cs.ignored = append(cs.ignored, logStreamName)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
