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
	if quiet {
		fmt.Println(output)
		return
	}

	windowWidth := 80

	if err := termbox.Init(); err == nil {
		windowWidth, _ = termbox.Size()
		termbox.Close()
	}

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
	WrongPlatformInstanceIds []string
}

func commandInstanceIds(sess *session.Session, commandId string) CommandInstanceIdsOutput {
	ssmClient := ssm.New(sess)
	listInput := &ssm.ListCommandInvocationsInput{CommandId: &commandId}

	resp3, err := ssmClient.ListCommandInvocations(listInput)
	if err != nil { log.Panicf(err.Error()) }

	invocationMap := map[string]*ssm.CommandInvocation{}

	instanceIds := []string{}
	for _, invocation := range resp3.CommandInvocations {
		id := *invocation.InstanceId
		instanceIds = append(instanceIds, id)
		invocationMap[id] = invocation
	}

	ec2client := ec2.New(sess)
	instanceDescs, _ := ec2client.DescribeInstances(&ec2.DescribeInstancesInput{
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
		InstanceIds: instanceIds,
		FaultyInstanceIds: faulty,
		WrongPlatformInstanceIds: wrongInstanceIds,
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

func makeTargets(tagPairs, instanceIds []string) []*ssm.Target {
	targets := []*ssm.Target{}

	for _, pair := range tagPairs {
		splitted := strings.SplitN(pair, "=", 2)

		tag := splitted[0]
		val := splitted[1]
		key := fmt.Sprintf("tag:%s", tag)

		target := &ssm.Target{
			Key: &key,
			Values: []*string{&val},
		}
		targets = append(targets, target)
	}

	if len(instanceIds) > 0 {
		target := &ssm.Target{
			Key: aws.String("InstanceIds"),
			Values: aws.StringSlice(instanceIds),
		}
		targets = append(targets, target)
	}

	return targets
}

func makeCommandInput(targets []*ssm.Target, bucket, keyPrefix, command, shellType string, timeout int64) *ssm.SendCommandInput {
	docName := "AWS-RunShellScript"
	if shellType == "powershell" {
		docName = "AWS-RunPowerShellScript"
	}

	return &ssm.SendCommandInput{
		DocumentName: &docName,
		Targets: targets,
		TimeoutSeconds: &timeout,
		OutputS3BucketName: &bucket,
		OutputS3KeyPrefix: &keyPrefix,
		Parameters: map[string][]*string {
			"commands": aws.StringSlice([]string{command}),
		},
	}
}

func printInfo(prefix, info string) {
	if !quiet {
		fmt.Printf("%s%s\n", color.BlueString("%s", prefix), color.New(color.Faint).Sprintf("%s", info))
	}
}

func doit(sess *session.Session, commandInput *ssm.SendCommandInput) {
	client := ssm.New(sess)

	resp, err := client.SendCommand(commandInput)

	if err != nil {
		log.Panicf(err.Error())
	}

	time.Sleep(time.Second * 3)

	commandId := resp.Command.CommandId
	instanceIds := commandInstanceIds(sess, *commandId)
	printedInstanceIds := []string{}

	if !quiet {
		printInfo("Command ID: ", *commandId)
		printInfo(fmt.Sprintf("Running command on %d instances: ", len(instanceIds.InstanceIds)), fmt.Sprintf("%+v", instanceIds.InstanceIds))
		if len(instanceIds.FaultyInstanceIds) > 0 {
			color.Red("Command sent to %d terminated instances: %+v\n", len(instanceIds.FaultyInstanceIds), instanceIds.FaultyInstanceIds)
		}
		if len(instanceIds.WrongPlatformInstanceIds) > 0 {
			color.Red("Command sent to %d wrong OS instances: %+v\n", len(instanceIds.WrongPlatformInstanceIds), instanceIds.WrongPlatformInstanceIds)
		}
	}

	expectedResponseCount := len(instanceIds.InstanceIds) - len(instanceIds.FaultyInstanceIds) - len(instanceIds.WrongPlatformInstanceIds)

	for {
		time.Sleep(time.Second * 3)

		listInput := &ssm.ListCommandInvocationsInput{CommandId: commandId}
		resp3, err := client.ListCommandInvocations(listInput)

		if err != nil {
			log.Panicf(err.Error())
		}

		for _, invocation := range resp3.CommandInvocations {
			instanceId := *invocation.InstanceId
			if stringInSlice(instanceId, printedInstanceIds) ||
				stringInSlice(instanceId, instanceIds.FaultyInstanceIds) ||
				stringInSlice(instanceId, instanceIds.WrongPlatformInstanceIds) {
				continue
			}

			if *invocation.Status != "InProgress" {
				time.Sleep(time.Second * 3)

				prefix := fmt.Sprintf("[%d/%d %s] ", len(printedInstanceIds) + 1, expectedResponseCount, instanceId)

				colourIdx := len(printedInstanceIds) % 2

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

				if !quiet {
					colour := color.New(color.FgBlue)
					message := fmt.Sprintf("%s: %s", *invocation.Status, *invocation.StatusDetails)
					printFormattedOutput(colour.Sprint(prefix), message)
				}

				printedInstanceIds = append(printedInstanceIds, instanceId)
			}
		}

		if len(printedInstanceIds) == expectedResponseCount {
			break
		}
	}
}
