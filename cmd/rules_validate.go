package cmd

import (
	"fmt"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/rules"
	"github.com/spf13/cobra"
)

func newRulesValidateCmd() *cobra.Command {
	var tenant string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "校验租户规则",
		Long: `校验 rules/tenants/<tenant>/rules 是否满足当前规则契约。

会检查 package.yaml、input.yaml、generation-config.yaml、sections/*.yaml 和模板文件是否齐全，并验证当前 runtime-native 规则契约是否合法。
成功时 stdout 会输出“规则校验通过: tenant=<tenant>”。`,
		Example: `  syl-listing-pro-x rules validate --tenant syl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := rules.Service{Root: paths.RulesRepo}
			if err := svc.Validate(tenant); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "规则校验通过: tenant=%s\n", tenant)
			return nil
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "租户 ID")
	_ = cmd.MarkFlagRequired("tenant")
	return cmd
}
