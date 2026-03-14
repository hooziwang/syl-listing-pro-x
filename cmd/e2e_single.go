package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/e2e"
	"github.com/spf13/cobra"
)

func newE2ESingleCmd() *cobra.Command {
	var tenant string
	var key string
	var adminToken string
	var privateKeyPath string
	var inputPath string
	var outputDir string
	var workerURL string
	var artifactsID string
	var printPathContext bool
	defaultKey := loadUserEnvValue(".syl-listing-pro", "SYL_LISTING_KEY")
	defaultAdminToken := loadUserEnvValue(".syl-listing-pro-x", "ADMIN_TOKEN")

	cmd := &cobra.Command{
		Use:   "single",
		Short: "执行单文件真实回归验收",
		Long: `单文件真实回归验收入口，固定执行 single-listing-compliance-gate。

它会先发布规则，再诊断 worker，最后调用 syl-listing-pro，并对前台 verbose 异常和单文件 listing 细规则做验收。
传 --print-path-context 时，会在规则发布阶段把路径上下文打印到 stderr。
如果不传 --out，会默认落到 syl-listing-pro-x/out/<artifacts-id>。`,
		Example: `  syl-listing-pro-x e2e single --tenant syl --input /abs/demo.md
  syl-listing-pro-x e2e single --tenant syl --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e single --tenant syl --input /abs/demo.md --artifacts-id single-listing-regression-demo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(tenant) == "" {
				return fmt.Errorf("缺少 tenant：请传 --tenant")
			}
			if strings.TrimSpace(inputPath) == "" {
				return fmt.Errorf("缺少输入文件：请传 --input")
			}

			resolvedKey := strings.TrimSpace(key)
			if resolvedKey == "" {
				resolvedKey = loadUserEnvValue(".syl-listing-pro", "SYL_LISTING_KEY")
			}
			if resolvedKey == "" {
				return fmt.Errorf("缺少 SYL_LISTING_KEY：请传 --key，或在 ~/.syl-listing-pro/.env 中设置 SYL_LISTING_KEY=...")
			}
			resolvedAdminToken := strings.TrimSpace(adminToken)
			if resolvedAdminToken == "" {
				resolvedAdminToken = loadUserEnvValue(".syl-listing-pro-x", "ADMIN_TOKEN")
			}
			if resolvedAdminToken == "" {
				return fmt.Errorf("缺少 ADMIN_TOKEN：请传 --admin-token，或在 ~/.syl-listing-pro-x/.env 中设置 ADMIN_TOKEN=...")
			}

			resolvedArtifactsID, resolvedOutputDir := resolveSingleE2EPaths(artifactsID, outputDir)
			resolvedPrivateKeyPath, restoreEnv := resolveSingleE2EPrivateKey(privateKeyPath)
			defer restoreEnv()
			svc := newE2ERunner(cmd.ErrOrStderr())
			result, err := svc.Run(cmd.Context(), e2e.RunInput{
				CaseName:         "single-listing-compliance-gate",
				Tenant:           tenant,
				SYLKey:           resolvedKey,
				AdminToken:       resolvedAdminToken,
				PrivateKeyPath:   resolvedPrivateKeyPath,
				InputPath:        inputPath,
				OutputDir:        resolvedOutputDir,
				WorkerURL:        workerURL,
				ArtifactsID:      resolvedArtifactsID,
				PrintPathContext: printPathContext,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), result.ArtifactsDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&tenant, "tenant", "", "租户 ID")
	cmd.Flags().StringVar(&key, "key", defaultKey, "SYL_LISTING_KEY，默认从 ~/.syl-listing-pro/.env 读取")
	cmd.Flags().StringVar(&adminToken, "admin-token", defaultAdminToken, "ADMIN_TOKEN，默认从 ~/.syl-listing-pro-x/.env 读取")
	cmd.Flags().StringVar(&privateKeyPath, "private-key", "", "规则签名私钥路径；不传则按环境变量解析")
	cmd.Flags().StringVar(&inputPath, "input", "", "输入 markdown 文件")
	cmd.Flags().StringVar(&outputDir, "out", "", "输出目录；不传则自动推导")
	cmd.Flags().StringVar(&workerURL, "worker", paths.WorkerURL, "worker 地址")
	cmd.Flags().StringVar(&artifactsID, "artifacts-id", "", "artifacts 目录名；不传则自动生成")
	cmd.Flags().BoolVar(&printPathContext, "print-path-context", false, "在规则发布阶段把路径上下文打印到 stderr")
	return cmd
}

func resolveSingleE2EPaths(artifactsID string, outputDir string) (string, string) {
	resolvedArtifactsID := strings.TrimSpace(artifactsID)
	if resolvedArtifactsID == "" {
		resolvedArtifactsID = "single-listing-regression-" + time.Now().UTC().Format("20060102-150405")
	}
	resolvedOutputDir := strings.TrimSpace(outputDir)
	if resolvedOutputDir == "" {
		resolvedOutputDir = filepath.Join(paths.WorkspaceRoot, "syl-listing-pro-x", "out", resolvedArtifactsID)
	}
	return resolvedArtifactsID, resolvedOutputDir
}

func resolveSingleE2EPrivateKey(explicit string) (string, func()) {
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
