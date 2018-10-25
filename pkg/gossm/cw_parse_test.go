package gossm

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestLogEventToSsmMessageStdout(t *testing.T) {
	event := &cloudwatchlogs.FilteredLogEvent{
		EventId: aws.String("34354377445391684770582427282234606724320249752192286720"),
		IngestionTime: aws.Int64(1540503563531),
		Timestamp: aws.Int64(1540503563426),
		LogStreamName: aws.String("7017174d-d284-486e-9dcb-22221883be98/i-02a6983afe9f00d21/aws-runShellScript/stdout"),
		Message: aws.String(" 21:39:20 up 3 days, 23:43,  0 users,  load average: 0.08, 0.03, 0.01"),
	}
	msg := logEventToSsmMessage(event)
	assert.Equal(t, msg.InstanceId, "i-02a6983afe9f00d21")
	assert.Equal(t, msg.CommandId, "7017174d-d284-486e-9dcb-22221883be98")
	assert.Equal(t, msg.StdoutChunk, " 21:39:20 up 3 days, 23:43,  0 users,  load average: 0.08, 0.03, 0.01")
	assert.Equal(t, msg.StderrChunk, "")

}

func TestLogEventToSsmMessageStderr(t *testing.T) {
	event := &cloudwatchlogs.FilteredLogEvent{
		EventId: aws.String("34354377445391684770582427282234606724320249752192286720"),
		IngestionTime: aws.Int64(1540503563531),
		Timestamp: aws.Int64(1540503563426),
		LogStreamName: aws.String("7017174d-d284-486e-9dcb-22221883be98/i-02a6983afe9f00d21/aws-runShellScript/stderr"),
		Message: aws.String(" 21:39:20 up 3 days, 23:43,  0 users,  load average: 0.08, 0.03, 0.01"),
	}
	msg := logEventToSsmMessage(event)
	assert.Equal(t, msg.InstanceId, "i-02a6983afe9f00d21")
	assert.Equal(t, msg.CommandId, "7017174d-d284-486e-9dcb-22221883be98")
	assert.Equal(t, msg.StderrChunk, " 21:39:20 up 3 days, 23:43,  0 users,  load average: 0.08, 0.03, 0.01")
	assert.Equal(t, msg.StdoutChunk, "")
}
