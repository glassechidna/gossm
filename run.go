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
	"github.com/aws/aws-sdk-go/service/ec2"
	"text/template"
	"bytes"
	"github.com/aws/aws-sdk-go/service/sts"
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

type CommandInstanceIdsOutput struct {
	InstanceIds []string
	FaultyInstanceIds []string
}

func commandInstanceIds(sess *session.Session, commandId string) CommandInstanceIdsOutput {
	ssmClient := ssm.New(sess)
	listInput := &ssm.ListCommandInvocationsInput{CommandId: &commandId}

	resp3, err := ssmClient.ListCommandInvocations(listInput)
	if err != nil { log.Panicf(err.Error()) }

	instanceIds := []string{}
	for _, invocation := range resp3.CommandInvocations {
		instanceIds = append(instanceIds, *invocation.InstanceId)
	}

	ec2client := ec2.New(sess)
	instanceDescs, _ := ec2client.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice(instanceIds),
	})

	liveInstanceIds := []string{}
	for _, reservation := range instanceDescs.Reservations {
		for _, instance := range reservation.Instances {
			if *instance.State.Name == "running" {
				liveInstanceIds = append(liveInstanceIds, *instance.InstanceId)
			}
		}
	}

	faulty := []string{}

	for _, instanceId := range instanceIds {
		if !stringInSlice(instanceId, liveInstanceIds) {
			faulty = append(faulty, instanceId)
		}
	}

	return CommandInstanceIdsOutput{
		InstanceIds: instanceIds,
		FaultyInstanceIds: faulty,
	}
}

func realBucketName(sess *session.Session, input string) string {
	tmpl, err := template.New("bucket").Parse(input)
	if err != nil { log.Panicf(err.Error()) }

	buf := bytes.Buffer{}

	region := *sess.Config.Region
	identity, _ := sts.New(sess).GetCallerIdentity(&sts.GetCallerIdentityInput{})
	accountId := *identity.Account

	err = tmpl.Execute(&buf, map[string]string{
		"Region": region,
		"AccountId": accountId,
	})
	if err != nil { log.Panicf(err.Error()) }

	return buf.String()
}

func doit(sess *session.Session, targets []*ssm.Target, bucket, keyPrefix, command string, timeout int64) {
	client := ssm.New(sess)
	bucket = realBucketName(sess, bucket)

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

	time.Sleep(time.Second * 3)

	commandId := resp.Command.CommandId
	instanceIds := commandInstanceIds(sess, *commandId)
	printedInstanceIds := []string{}

	color.Blue("Running command on instances: %+v\n", instanceIds.InstanceIds)
	if len(instanceIds.FaultyInstanceIds) > 0 {
		color.Red("Command sent to terminated instances: %+v\n", instanceIds.FaultyInstanceIds)
	}
	expectedResponseCount := len(instanceIds.InstanceIds) - len(instanceIds.FaultyInstanceIds)

	for {
		time.Sleep(time.Second * 3)

		listInput := &ssm.ListCommandInvocationsInput{CommandId: commandId}
		resp3, err := client.ListCommandInvocations(listInput)

		if err != nil {
			log.Panicf(err.Error())
		}

		for _, invocation := range resp3.CommandInvocations {
			instanceId := *invocation.InstanceId
			if stringInSlice(instanceId, printedInstanceIds) {
				continue
			}

			if *invocation.Status != "InProgress" {
				time.Sleep(time.Second * 3)

				colourIdx := len(printedInstanceIds) % 2
				prefix := fmt.Sprintf("[%d/%d %s] ", len(printedInstanceIds) + 1, expectedResponseCount, instanceId)

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

		if len(printedInstanceIds) == expectedResponseCount {
			break
		}
	}
}
