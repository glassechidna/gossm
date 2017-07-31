package main

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"log"
	"time"
	"os"
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

func doit(sess *session.Session, instanceId, command string, timeout int64) {
	client := ssm.New(sess)
	resp, err := client.SendCommand(&ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds: aws.StringSlice([]string{instanceId}),
		TimeoutSeconds: &timeout,
		Parameters: map[string][]*string {
			"commands": aws.StringSlice([]string{command}),
		},
	})

	if err != nil {
		log.Panicf(err.Error())
	}

	commandId := resp.Command.CommandId
	status := "InProgress"

	cmdResp := ssm.GetCommandInvocationOutput{}

	for {
		time.Sleep(time.Second * 3)

		resp2, err := client.GetCommandInvocation(&ssm.GetCommandInvocationInput{
			CommandId: commandId,
			InstanceId: &instanceId,
		})

		if err != nil {
			log.Panicf(err.Error())
		}

		status = *resp2.Status

		if status != "InProgress" {
			cmdResp = *resp2
			break
		}
	}

	os.Stderr.WriteString(*cmdResp.StandardErrorContent)
	os.Stdout.WriteString(*cmdResp.StandardOutputContent)
}
