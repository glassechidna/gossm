package gossmcmd

import (
	"fmt"
	"github.com/glassechidna/gossm/actions"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
	"log"
	"time"
)

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Run gossm web UI",
	Run: func(cmd *cobra.Command, args []string) {
		showBrowser, _ := cmd.PersistentFlags().GetBool("no-open")
		showBrowser = !showBrowser

		port, _ := cmd.PersistentFlags().GetInt("port")

		if showBrowser {
			time.AfterFunc(time.Second, func() {
				_ = open.Start(fmt.Sprintf("http://localhost:%d", port))
			})
		}

		app := actions.App()
		app.Options.Addr = fmt.Sprintf(":%d", port)
		if err := app.Serve(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	RootCmd.AddCommand(browserCmd)
	browserCmd.PersistentFlags().Bool("no-open", false, "")
	browserCmd.PersistentFlags().Int("port", 3000, "")

}
