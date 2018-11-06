package gossmcmd

import (
	"fmt"
	"github.com/glassechidna/gossm/pkg/awssess"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
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

		doit(sess, shell, command, files, quiet, timeout, tagPairs, instanceIds)
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
