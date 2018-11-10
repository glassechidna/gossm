package cwlogs

import (
	"context"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"math"
	"strings"
	"sync"
	"time"
)

type CwStream struct {
	Input           *cloudwatchlogs.FilterLogEventsInput
	Sleep           time.Duration
	ignored         []string
	ignoredPrefixes []string
	mut             *sync.Mutex
}

func (c *CwStream) Stream(ctx context.Context, api cloudwatchlogsiface.CloudWatchLogsAPI, ch chan *cloudwatchlogs.FilteredLogEvent) {
	lastTimestamps := map[string]int64{}
	seenEvents := map[string]bool{}

	if c == nil || c.Input == nil || c.Input.LogGroupName == nil {
		panic("What are you trying to stream? You need to specify at least a log group name")
	}

	if c.mut == nil {
		c.mut = &sync.Mutex{}
	}

	if c.Sleep == 0 {
		c.Sleep = time.Second
	}

	for {
		time.Sleep(c.Sleep)

		err := api.FilterLogEventsPagesWithContext(ctx, c.Input, func(page *cloudwatchlogs.FilterLogEventsOutput, lastPage bool) bool {
			for _, event := range page.Events {
				if c.shouldIgnore(event) {
					continue
				}

				if _, ok := seenEvents[*event.EventId]; !ok {
					ch <- event
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

		c.mut.Lock()
		for logStreamName, ts := range lastTimestamps {
			if ts < lastTimestamp && !stringInSlice(logStreamName, c.ignored) {
				lastTimestamp = ts
			}
		}

		c.mut.Unlock()
		lastTimestamp++
		c.Input.StartTime = &lastTimestamp

		if ctx.Err() == context.Canceled {
			close(ch)
			break
		}
	}

}

func (c *CwStream) shouldIgnore(event *cloudwatchlogs.FilteredLogEvent) bool {
	if stringInSlice(*event.LogStreamName, c.ignored) {
		return true
	}
	for _, ignoredPrefix := range c.ignoredPrefixes {
		if strings.HasPrefix(*event.LogStreamName, ignoredPrefix) {
			return true
		}
	}
	return false
}

func (c *CwStream) Ignore(logStreamName string) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.ignored = append(c.ignored, logStreamName)
}

func (c *CwStream) IgnorePrefix(logStreamNamePrefix string) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.ignoredPrefixes = append(c.ignoredPrefixes, logStreamNamePrefix)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
