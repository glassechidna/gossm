package gossmcmd

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/glassechidna/awscredcache"
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/glassechidna/gossm/pkg/gossm/printer"
	"github.com/pquerna/otp/totp"
	"io"
	"os"
	"path/filepath"
	"time"
)

func doit(sess *session.Session, shellType, command string, files, quiet bool, timeout int64, tagPairs, instanceIds []string) {
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
		CloudWatchOutputConfig: &ssm.CloudWatchOutputConfig{
			CloudWatchOutputEnabled: aws.Bool(true),
		},
		Parameters: map[string][]*string{
			"commands": aws.StringSlice([]string{command}),
		},
	}

	resp, err := client.Doit(context.Background(), input)
	if err != nil {
		panic(err)
	}

	printer.PrintInfo(command, resp)

	ch := make(chan gossm.SsmMessage)
	go client.Poll(context.Background(), resp.CommandId, ch)

	if files {
		printToFiles(resp, ch)
	} else {
		for msg := range ch {
			printer.Print(msg)
		}
	}
}

func printToFiles(resp *gossm.DoitResponse, ch chan gossm.SsmMessage) {
	dir, err := filepath.Abs(resp.CommandId)
	if err != nil {
		panic(err)
	}

	err = os.Mkdir(dir, 0755)
	if err != nil {
		panic(err)
	}

	files := map[string]io.WriteCloser{}
	for _, id := range resp.InstanceIds.InstanceIds {
		fpath := filepath.Join(dir, fmt.Sprintf("%s.txt", id))
		file, err := os.Create(fpath)
		if err != nil {
			panic(err)
		}
		files[id] = file
	}

	for msg := range ch {
		if msg.Payload == nil {
			continue
		}
		file := files[msg.Payload.InstanceId]
		_, err = file.Write([]byte(msg.Payload.StdoutChunk))
		if err != nil {
			panic(err)
		}
		_, err = file.Write([]byte(msg.Payload.StderrChunk))
		if err != nil {
			panic(err)
		}
	}

	for _, file := range files {
		err = file.Close()
		if err != nil {
			panic(err)
		}
	}
}

func AwsSession(profile, region string) *session.Session {
	provider := awscredcache.NewAwsCacheCredProvider(profile)
	provider.MfaCodeProvider = func(mfaSecret string) (string, error) {
		if len(mfaSecret) == 0 {
			return stscreds.StdinTokenProvider()
		} else {
			return totp.GenerateCode(mfaSecret, time.Now())
		}
	}

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
	//sess.Config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)

	if len(region) > 0 {
		sess.Config.Region = &region
	}

	return sess
}
