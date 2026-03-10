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
