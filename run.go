package main

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"log"
	"time"
	"net/url"
	"github.com/aws/aws-sdk-go/service/s3"
	"strings"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"fmt"
	"github.com/mitchellh/go-wordwrap"
	"github.com/fatih/color"
	"github.com/nsf/termbox-go"
)

func AwsSession(profile, region string) *session.Session {
	sessOpts := session.Options{
		SharedConfigState: session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
	}

	if len(profile) > 0 {
		sessOpts.Profile = profile
	}

	sess, _ := session.NewSessionWithOptions(sessOpts)
	config := aws.NewConfig()

	if len(region) > 0 {
		config.Region = aws.String(region)
		sess.Config = config
	}

	return sess
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getFromS3Url(sess *session.Session, urlString string) (*string, error) {
	s3url, err := url.Parse(urlString)
	if err != nil { return nil, err }

	urlPath := s3url.Path
	parts := strings.SplitN(urlPath, "/", 3)
	bucket := parts[1]
	key := parts[2]

	buff := &aws.WriteAtBuffer{}
	s3dl := s3manager.NewDownloader(sess)

	_, err = s3dl.Download(buff, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil { return nil, err }

	str := string(buff.Bytes())
	return &str, nil
}

func printFormattedOutput(prefix, output string) {
	if err := termbox.Init(); err != nil { panic(err) }
	windowWidth, _ := termbox.Size()
	termbox.Close()

	outputWidth := windowWidth - len(prefix)
	wrapped := wordwrap.WrapString(output, uint(outputWidth))
	lines := strings.Split(wrapped, "\n")

	for _, line := range(lines) {
		fmt.Print(prefix)
		fmt.Println(line)
	}
}

var stdoutColours = []*color.Color{
	color.New(color.FgGreen),
	color.New(color.FgHiGreen),
}
var stderrColours = []*color.Color{
	color.New(color.FgRed),
	color.New(color.FgHiRed),
}

func doit(sess *session.Session, targets []*ssm.Target, bucket, keyPrefix, command string, timeout int64) {
	client := ssm.New(sess)
	resp, err := client.SendCommand(&ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Targets: targets,
		TimeoutSeconds: &timeout,
		OutputS3BucketName: &bucket,
		OutputS3KeyPrefix: &keyPrefix,
		Parameters: map[string][]*string {
			"commands": aws.StringSlice([]string{command}),
		},
	})

	if err != nil {
		log.Panicf(err.Error())
	}

	commandId := resp.Command.CommandId
	printedInstanceIds := []string{}


	for {
		time.Sleep(time.Second * 3)

		resp3, err := client.ListCommandInvocations(&ssm.ListCommandInvocationsInput{
			CommandId: commandId,
		})

		if err != nil {
			log.Panicf(err.Error())
		}

		for _, invocation := range resp3.CommandInvocations {
			instanceId := *invocation.InstanceId
			if stringInSlice(instanceId, printedInstanceIds) {
				continue
			}

			if *invocation.Status != "InProgress" {
				colourIdx := len(printedInstanceIds) % 2
				prefix := fmt.Sprintf("[%d/%d %s] ", len(printedInstanceIds) + 1, len(resp3.CommandInvocations), instanceId)

				stdout, _ := getFromS3Url(sess, *invocation.StandardOutputUrl)
				if stdout != nil {
					colour := stdoutColours[colourIdx]
					printFormattedOutput(colour.Sprint(prefix), *stdout)
				}

				stderr, _ := getFromS3Url(sess, *invocation.StandardErrorUrl)
				if stderr != nil {
					colour := stderrColours[colourIdx]
					printFormattedOutput(colour.Sprint(prefix), *stderr)
				}

				printedInstanceIds = append(printedInstanceIds, instanceId)
			}
		}

		if len(printedInstanceIds) == len(resp3.CommandInvocations) {
			break
		}
	}
}
