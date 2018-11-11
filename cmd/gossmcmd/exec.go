package gossmcmd

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/glassechidna/gossm/pkg/awssess"
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/glassechidna/gossm/pkg/gossm/printer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var quiet bool

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Run commands on remote machines using EC2 SSM Run Command",
	Run: func(cmd *cobra.Command, args []string) {
		region := viper.GetString("region")
		profile := viper.GetString("profile")
		sess := awssess.AwsSession(profile, region)

		instanceIds, _ := cmd.PersistentFlags().GetStringSlice("instance-id")
		tagPairs, _ := cmd.PersistentFlags().GetStringSlice("tag")

		timeout, _ := cmd.PersistentFlags().GetInt64("timeout")
		command := getCommandInput(args)

		shell := "bash"
		if viper.GetBool("powershell") {
			shell = "powershell"
		}
		files := viper.GetBool("files")

		doit(sess, gossm.DefaultHistory, shell, command, files, quiet, timeout, tagPairs, instanceIds)
	},
}

func getCommandInput(argv []string) string {
	command := strings.Join(argv, " ")

	if len(command) == 0 {
		fmt.Println("Enter command (and then hit Ctrl+D):")
		bytes, _ := ioutil.ReadAll(os.Stdin)
		command = string(bytes)
	}

	return command
}

func doit(sess *session.Session, history *gossm.History, shellType, command string, files, quiet bool, timeout int64, tagPairs, instanceIds []string) {
	client := gossm.New(sess, history)

	printer := printer.New()
	printer.Quiet = quiet

	docName := "AWS-RunShellScript"
	if shellType == "powershell" {
		docName = "AWS-RunPowerShellScript"
	}

	targets := gossm.MakeTargets(tagPairs, instanceIds)

	input := &ssm.SendCommandInput{
		DocumentName:   &docName,
		Targets:        targets,
		TimeoutSeconds: &timeout,
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

	printer.PrintInfo(resp)

	ch := make(chan gossm.SsmMessage)
	go client.Poll(context.Background(), *resp.Command.CommandId, ch)

	if files {
		printToFiles(resp, ch)
	} else {
		for msg := range ch {
			printer.Print(msg)
		}
	}
}

func printToFiles(resp *gossm.Status, ch chan gossm.SsmMessage) {
	dir, err := filepath.Abs(*resp.Command.CommandId)
	if err != nil {
		panic(err)
	}

	err = os.Mkdir(dir, 0755)
	if err != nil {
		panic(err)
	}

	files := map[string]io.WriteCloser{}
	for _, id := range resp.Invocations.InstanceIds() {
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

func init() {
	RootCmd.AddCommand(execCmd)

	execCmd.PersistentFlags().String("profile", "", "")
	execCmd.PersistentFlags().String("region", "", "")
	execCmd.PersistentFlags().BoolP("powershell", "p", false, "")
	execCmd.PersistentFlags().BoolP("files", "f", false, "")
	execCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "")
	execCmd.PersistentFlags().StringSliceP("instance-id", "i", []string{}, "")
	execCmd.PersistentFlags().StringSliceP("tag", "t", []string{}, "")
	execCmd.PersistentFlags().Int64("timeout", 600, "")

	viper.BindPFlags(execCmd.PersistentFlags())
}
