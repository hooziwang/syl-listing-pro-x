package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/rules"
	"github.com/spf13/cobra"
)

func newRulesPublishCmd() *cobra.Command {
	var tenant string
	var version string
	var privateKey string
	var workerURL string
	var adminToken string
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "打包并发布租户规则",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := rules.Service{Root: paths.RulesRepo}
			if version == "" {
				version = rules.GenerateVersion(tenant)
			}
			if _, err := svc.Package(tenant, version, privateKey); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			resp, err := svc.Publish(ctx, rules.PublishInput{
				Tenant:     tenant,
				Version:    version,
				WorkerURL:  workerURL,
				AdminToken: adminToken,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", resp.RulesVersion)
			return nil
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "租户 ID")
	cmd.Flags().StringVar(&version, "version", "", "规则版本，不传则自动生成")
	cmd.Flags().StringVar(&privateKey, "private-key", "", "签名私钥路径；不传则按环境变量或开发模式解析")
	cmd.Flags().StringVar(&workerURL, "worker", paths.WorkerURL, "worker 地址")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "管理员令牌")
	_ = cmd.MarkFlagRequired("tenant")
	_ = cmd.MarkFlagRequired("admin-token")
	return cmd
}
