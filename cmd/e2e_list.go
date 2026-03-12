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
	Long: `列出当前支持的 e2e 用例名称。

当前包含 release-gate、architecture-gate、listing-compliance-gate 和 single-listing-compliance-gate，便于在跑 e2e run 之前先确认用例名。`,
		Example: `  syl-listing-pro-x e2e list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := e2e.NewDefaultService(paths)
			for _, name := range svc.ListCases() {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			return nil
		},
	}
}
