package cmd

import "github.com/spf13/cobra"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			printVersion(cmd.OutOrStdout())
		},
	}
}
