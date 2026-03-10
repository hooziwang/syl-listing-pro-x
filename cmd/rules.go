package cmd

import "github.com/spf13/cobra"

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "规则工具链",
	}
	cmd.AddCommand(newRulesValidateCmd())
	cmd.AddCommand(newRulesPackageCmd())
	cmd.AddCommand(newRulesPublishCmd())
	return cmd
}
