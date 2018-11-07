package gossmcmd

import (
	"fmt"
	"github.com/apcera/termtables"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/dustin/go-humanize"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show previously-executed commands",
	Run: func(cmd *cobra.Command, args []string) {
		h := defaultHistory()
		cmds, err := h.Commands()
		if err != nil {
			panic(err)
		}

		table := termtables.CreateTable()
		table.AddHeaders("Timestamp", "Command ID", "Status", "Command")

		for _, cmd := range cmds {
			input := *cmd.Command.Parameters["commands"][0]
			timestamp := humanize.Time(*cmd.Command.RequestedDateTime)
			cmdId := *cmd.Command.CommandId
			total := *cmd.Command.TargetCount
			var success int64 = 0

			for _, inv := range cmd.Invocations {
				if *inv.Status == ssm.CommandInvocationStatusSuccess {
					success++
				}
			}

			icon := aurora.Green("✔")
			if success == 0 {
				icon = aurora.Red("✗")
			} else if success < total {
				icon = aurora.Cyan("!")
			}
			status := fmt.Sprintf("%s (%d/%d)", icon.String(), success, total)
			table.AddRow(timestamp, cmdId, status, input)
		}

		fmt.Println(table.Render())
	},
}

func init() {
	RootCmd.AddCommand(historyCmd)
}
