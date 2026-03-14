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
	var printPathContext bool
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "打包并发布租户规则",
		Long: `会先打包，再调用 worker 管理接口发布。

发布目标是 <worker>/v1/admin/tenant-rules/publish，因此要求 worker 管理接口可达且 --admin-token 有效。
成功时 stdout 只输出已生效的 rules_version。传 --print-path-context 时，stderr 打印本次解析到的路径上下文。`,
		Example: `  syl-listing-pro-x rules publish --tenant syl --admin-token <ADMIN_TOKEN>
  syl-listing-pro-x rules publish --tenant syl --admin-token <ADMIN_TOKEN> --worker https://worker.example.test
  syl-listing-pro-x rules publish --tenant syl --admin-token <ADMIN_TOKEN> --private-key /abs/rules.pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := rules.Service{Root: paths.RulesRepo}
			if version == "" {
				version = rules.GenerateVersion(tenant)
			}
			out, err := svc.Package(tenant, version, privateKey)
			if err != nil {
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
			if printPathContext {
				writeRulesCommandPathContext(cmd.ErrOrStderr(), "publish", rulesCommandPathContext{
					WorkspaceRoot: paths.WorkspaceRoot,
					RulesRepo:     paths.RulesRepo,
					PackageDir:    out.PackageDir,
					RulesVersion:  resp.RulesVersion,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", resp.RulesVersion)
			return nil
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "租户 ID")
	cmd.Flags().StringVar(&version, "version", "", "规则版本，不传则自动生成")
	cmd.Flags().StringVar(&privateKey, "private-key", "", "签名私钥路径；不传则按环境变量或开发模式解析")
	cmd.Flags().BoolVar(&printPathContext, "print-path-context", false, "把本次解析到的路径上下文打印到 stderr")
	cmd.Flags().StringVar(&workerURL, "worker", paths.WorkerURL, "worker 地址")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "管理员令牌")
	_ = cmd.MarkFlagRequired("tenant")
	_ = cmd.MarkFlagRequired("admin-token")
	return cmd
}
