package cmd

import (
	"fmt"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/e2e"
	"github.com/spf13/cobra"
)

func newE2EListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出可用 e2e 用例",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := e2e.NewDefaultService(paths)
			for _, name := range svc.ListCases() {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			return nil
		},
	}
}
