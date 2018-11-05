package gossmcmd

import (
	"github.com/glassechidna/gossm/actions"
	"github.com/spf13/cobra"
	"log"
)

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Run gossm web UI",
	Run: func(cmd *cobra.Command, args []string) {
		app := actions.App()
		if err := app.Serve(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	RootCmd.AddCommand(browserCmd)
}
