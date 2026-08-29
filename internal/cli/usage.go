package cli

import "github.com/spf13/cobra"

func usageCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "usage",
		Short: "Print the compact agent-facing command contract",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = cmd.OutOrStdout().Write([]byte(usageText))
		},
	}
}
