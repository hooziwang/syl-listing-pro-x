package cmd

import "github.com/spf13/cobra"

func newE2ECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "端到端验收工具",
	}
	cmd.AddCommand(newE2EListCmd())
	cmd.AddCommand(newE2ERunCmd())
	return cmd
}
