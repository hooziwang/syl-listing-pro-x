package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	defaultAdminToken := loadUserEnvValue(".syl-listing-pro-x", "ADMIN_TOKEN")
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "打包并发布租户规则",
		Long: `会先打包，再调用 worker 管理接口发布。

发布目标是 <worker>/v1/admin/tenant-rules/publish，因此要求 worker 管理接口可达且 admin token 有效。
如果本地没有显式传 --admin-token，会默认从 ~/.syl-listing-pro-x/.env 读取 ADMIN_TOKEN。
如果本地没有显式传 --private-key，且机器上也没有配置规则私钥环境变量，会自动回退到 rules 仓库内开发私钥并临时开启开发模式。
成功时 stdout 只输出已生效的 rules_version。传 --print-path-context 时，stderr 打印本次解析到的路径上下文。`,
		Example: `  syl-listing-pro-x rules publish --tenant syl
  syl-listing-pro-x rules publish --tenant syl --worker https://worker.example.test
  syl-listing-pro-x rules publish --tenant syl --private-key /abs/rules.pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedAdminToken := strings.TrimSpace(adminToken)
			if resolvedAdminToken == "" {
				resolvedAdminToken = loadUserEnvValue(".syl-listing-pro-x", "ADMIN_TOKEN")
			}
			if resolvedAdminToken == "" {
				return fmt.Errorf("缺少 ADMIN_TOKEN：请传 --admin-token，或在 ~/.syl-listing-pro-x/.env 中设置 ADMIN_TOKEN=...")
			}

			resolvedPrivateKey, restoreEnv := resolveRulesPublishPrivateKey(privateKey)
			defer restoreEnv()

			svc := rules.Service{Root: paths.RulesRepo}
			if version == "" {
				version = rules.GenerateVersion(tenant)
			}
			out, err := svc.Package(tenant, version, resolvedPrivateKey)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			resp, err := svc.Publish(ctx, rules.PublishInput{
				Tenant:     tenant,
				Version:    version,
				WorkerURL:  workerURL,
				AdminToken: resolvedAdminToken,
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
	cmd.Flags().StringVar(&adminToken, "admin-token", defaultAdminToken, "管理员令牌，默认从 ~/.syl-listing-pro-x/.env 读取")
	_ = cmd.MarkFlagRequired("tenant")
	return cmd
}

func resolveRulesPublishPrivateKey(explicit string) (string, func()) {
	if value := strings.TrimSpace(explicit); value != "" {
		return value, func() {}
	}
	for _, key := range []string{
		"SYL_LISTING_RULES_PRIVATE_KEY",
		"SIGNING_PRIVATE_KEY_PEM",
		"SIGNING_PRIVATE_KEY_BASE64",
	} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return "", func() {}
		}
	}

	previous, hadPrevious := os.LookupEnv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY")
	if strings.TrimSpace(previous) == "" {
		_ = os.Setenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY", "1")
		return filepath.Join(paths.RulesRepo, "keys", "rules_private.pem"), func() {
			if hadPrevious {
				_ = os.Setenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY", previous)
				return
			}
			_ = os.Unsetenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY")
		}
	}
	return filepath.Join(paths.RulesRepo, "keys", "rules_private.pem"), func() {}
}
