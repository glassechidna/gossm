package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strings"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/aws"
)

var cfgFile string

var RootCmd = &cobra.Command{
	Use:   "gossm",
	Short: "Run commands on remote machines using EC2 SSM Run Command",
	Run: func(cmd *cobra.Command, args []string) {
		region, _ := cmd.PersistentFlags().GetString("region")
		profile, _ := cmd.PersistentFlags().GetString("profile")
		sess := AwsSession(profile, region)

		bucket, _ := cmd.PersistentFlags().GetString("s3-bucket")
		keyPrefix, _ := cmd.PersistentFlags().GetString("s3-key-prefix")
		instanceIds, _ := cmd.PersistentFlags().GetStringSlice("instance-id")
		tagPairs, _ := cmd.PersistentFlags().GetStringSlice("tag")

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

		timeout, _ := cmd.PersistentFlags().GetInt64("timeout")
		command := strings.Join(args, " ")

		doit(sess, targets, bucket, keyPrefix, command, timeout)
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gossm.yaml)")

	RootCmd.PersistentFlags().String("profile", "", "")
	RootCmd.PersistentFlags().String("region", "", "")
	RootCmd.PersistentFlags().String("s3-bucket", "", "")
	RootCmd.PersistentFlags().String("s3-key-prefix", "", "")
	RootCmd.PersistentFlags().StringSlice("instance-ids", []string{}, "")
	RootCmd.PersistentFlags().StringSliceP("tag", "t", []string{}, "")
	RootCmd.PersistentFlags().Int64("timeout", 600, "")
}

func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".gossm") // name of config file (without extension)
	viper.AddConfigPath("$HOME")  // adding home directory as first search path
	viper.AutomaticEnv()          // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
