package gossmcmd

import (
	"fmt"
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

		for _, cmd := range cmds {
			input := *cmd.Command.Parameters["commands"][0]
			timestamp := cmd.Command.RequestedDateTime.String()
			fmt.Printf("%s | %s | %s\n", timestamp, aurora.Green(*cmd.Command.CommandId), input)
		}
	},
}

func init() {
	RootCmd.AddCommand(historyCmd)
}
