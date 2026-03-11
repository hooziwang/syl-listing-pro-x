package cmd

import (
	"fmt"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/rules"
	"github.com/spf13/cobra"
)

func newRulesPackageCmd() *cobra.Command {
	var tenant string
	var version string
	var privateKey string
	cmd := &cobra.Command{
		Use:   "package",
		Short: "打包租户规则",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := rules.Service{Root: paths.RulesRepo}
			if version == "" {
				version = rules.GenerateVersion(tenant)
			}
			out, err := svc.Package(tenant, version, privateKey)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out.PackageDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "租户 ID")
	cmd.Flags().StringVar(&version, "version", "", "规则版本，不传则自动生成")
	cmd.Flags().StringVar(&privateKey, "private-key", "", "签名私钥路径；不传则按环境变量或开发模式解析")
	_ = cmd.MarkFlagRequired("tenant")
	return cmd
}
