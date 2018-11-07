package gossmcmd

import (
	"fmt"
	"github.com/apcera/termtables"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/dustin/go-humanize"
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/glassechidna/gossm/pkg/gossm/printer"
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

		if len(args) == 0 {
			historyOverview(cmds)
		} else if len(args) == 1 {
			historyShow(cmds, args[0])
		}
	},
}

func historyOverview(cmds []gossm.HistoricalCommand) {
	table := termtables.CreateTable()
	table.AddHeaders("Timestamp", "Command ID", "Status", "Command")

	for idx := range cmds {
		cmd := cmds[len(cmds)-1-idx]
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
}

func historyShow(h *gossm.History, cmds []gossm.HistoricalCommand, cmdId string) {
	var theCmd gossm.HistoricalCommand

	for _, cmd := range cmds {
		cmd := cmd
		if *cmd.Command.CommandId == cmdId {
			theCmd = cmd
		}
	}

	printer := printer.New()
	printer.Quiet = quiet

	printer.PrintInfo(theCmd, nil)

	ch := make(chan gossm.SsmMessage)
	go theCmd.Stream()

	for msg := range ch {
		printer.Print(msg)
	}
}

func init() {
	RootCmd.AddCommand(historyCmd)
}
