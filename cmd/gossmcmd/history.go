package gossmcmd

import (
	"fmt"
	"github.com/apcera/termtables"
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
		h := &historyUi{gossm.DefaultHistory}

		if len(args) == 0 {
			h.overview()
		} else if len(args) == 1 {
			h.show(args[0])
		}
	},
}

type historyUi struct {
	*gossm.History
}

func (h *historyUi) overview() {
	cmds, _ := h.Commands()

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

func (h *historyUi) show(cmdId string) {
	cmds, _ := h.Commands()
	var status gossm.HistoricalStatus

	for _, c := range cmds {
		c := c
		if *c.Command.CommandId == cmdId {
			status = c
		}
	}

	printer := printer.New()
	printer.Quiet = quiet

	printer.PrintInfo(status.Status)

	ch := make(chan gossm.SsmMessage)
	outputs, _ := h.CommandOutputs(cmdId)
	go status.Stream(outputs, ch)

	for msg := range ch {
		printer.Print(msg)
	}
}

func init() {
	RootCmd.AddCommand(historyCmd)
}
