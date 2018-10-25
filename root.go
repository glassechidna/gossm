package main

import (
	"fmt"
	"github.com/glassechidna/gossm/pkg/gossm"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"strings"
)

var cfgFile string
var quiet bool

var RootCmd = &cobra.Command{
	Use:   "gossm",
	Short: "Run commands on remote machines using EC2 SSM Run Command",
	Run: func(cmd *cobra.Command, args []string) {
		region := viper.GetString("region")
		profile := viper.GetString("profile")
		sess := AwsSession(profile, region)

		bucket := viper.GetString("s3-bucket")
		bucket = gossm.RealBucketName(sess, bucket)
		//keyPrefix := viper.GetString("s3-key-prefix")

		instanceIds, _ := cmd.PersistentFlags().GetStringSlice("instance-id")
		tagPairs, _ := cmd.PersistentFlags().GetStringSlice("tag")

		timeout, _ := cmd.PersistentFlags().GetInt64("timeout")
		command := getCommandInput(args)

		shell := "bash"
		if viper.GetBool("powershell") {
			shell = "powershell"
		}

		doit(sess, shell, command, bucket, quiet, timeout, tagPairs, instanceIds)
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
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
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gossm.yaml)")

	RootCmd.PersistentFlags().String("profile", "", "")
	RootCmd.PersistentFlags().String("region", "", "")
	RootCmd.PersistentFlags().String("s3-bucket", "", "")
	RootCmd.PersistentFlags().String("s3-key-prefix", "", "")
	RootCmd.PersistentFlags().BoolP("powershell", "p", false, "")
	RootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "")
	RootCmd.PersistentFlags().StringSliceP("instance-id", "i", []string{}, "")
	RootCmd.PersistentFlags().StringSliceP("tag", "t", []string{}, "")
	RootCmd.PersistentFlags().Int64("timeout", 600, "")

	viper.BindPFlags(RootCmd.PersistentFlags())
}

func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".gossm") // name of config file (without extension)
	viper.AddConfigPath("$HOME")  // adding home directory as first search path
	viper.AutomaticEnv()          // read in environment variables that match

	// If a config file is found, read it in.
	viper.ReadInConfig()
}
