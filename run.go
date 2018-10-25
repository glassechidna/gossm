package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/glassechidna/awscredcache"
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/glassechidna/gossm/pkg/gossm/printer"
)

func AwsSession(profile, region string) *session.Session {
	provider := awscredcache.NewAwsCacheCredProvider(profile)
	chain := provider.WrapInChain()
	creds := credentials.NewCredentials(chain)

	sessOpts := session.Options{
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		Config:                  aws.Config{Credentials: creds},
	}

	if len(profile) > 0 {
		sessOpts.Profile = profile
	}

	sess, _ := session.NewSessionWithOptions(sessOpts)

	if len(region) > 0 {
		sess.Config.Region = &region
	}

	return sess
}

func doit(sess *session.Session, shellType, command, bucket string, quiet bool, timeout int64, tagPairs, instanceIds []string) {
	client := gossm.New(sess)

	printer := printer.New()
	printer.Quiet = quiet

	docName := "AWS-RunShellScript"
	if shellType == "powershell" {
		docName = "AWS-RunPowerShellScript"
	}

	targets := gossm.MakeTargets(tagPairs, instanceIds)

	input := &ssm.SendCommandInput{
		DocumentName:       &docName,
		Targets:            targets,
		TimeoutSeconds:     &timeout,
		OutputS3BucketName: &bucket,
		//OutputS3KeyPrefix: &keyPrefix,
		Parameters: map[string][]*string{
			"commands": aws.StringSlice([]string{command}),
		},
	}

	resp, err := client.Doit(input)
	if err != nil {
		panic(err)
	}

	printer.PrintInfo(command, resp)

	for msg := range resp.Channel {
		printer.Print(msg)
	}
}
