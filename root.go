package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strings"
)

var cfgFile string

var RootCmd = &cobra.Command{
	Use:   "gossm",
	Short: "Run commands on remote machines using EC2 SSM Run Command",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		region, _ := cmd.PersistentFlags().GetString("region")
		profile, _ := cmd.PersistentFlags().GetString("profile")
		sess := AwsSession(profile, region)

		instance, _ := cmd.PersistentFlags().GetString("instance-id")
		timeout, _ := cmd.PersistentFlags().GetInt64("timeout")
		command := strings.Join(args, " ")

		doit(sess, instance, command, timeout)
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
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	RootCmd.PersistentFlags().String("profile", "", "")
	RootCmd.PersistentFlags().String("region", "", "")
	RootCmd.PersistentFlags().String("instance-id", "", "")
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
