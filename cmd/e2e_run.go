package cmd

import (
	"fmt"
	"strings"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/e2e"
	"github.com/spf13/cobra"
)

func newE2ERunCmd() *cobra.Command {
	var caseName string
	var tenant string
	var key string
	var adminToken string
	var privateKeyPath string
	var inputPath string
	var outputDir string
	var workerURL string
	var artifactsID string
	defaultKey := loadUserEnvValue(".syl-listing-pro", "SYL_LISTING_KEY")
	defaultAdminToken := loadUserEnvValue(".syl-listing-pro-x", "ADMIN_TOKEN")
	cmd := &cobra.Command{
		Use:   "run",
		Short: "执行 e2e 用例",
	Long: `执行真实发布验收。会先发布规则，再诊断 worker，最后调用 syl-listing-pro。

这个命令依赖 PATH 中可执行的 syl-listing-pro，并会在运行时把 CLI stdout、stderr 和 verbose 日志写入 artifacts 目录。stdout 只打印 artifacts 目录路径。
可用用例：release-gate, architecture-gate, listing-compliance-gate, single-listing-compliance-gate。
其中 listing-compliance-gate 会固定使用 syl-listing-pro-x/testdata/e2e 下的输入样例，不要求传 --input；single-listing-compliance-gate 会对单个输入做真实执行、verbose 异常检查和 listing 细规则校验。`,
		Example: `  syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e run --case architecture-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --private-key /abs/rules.pem --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e run --case listing-compliance-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --out /abs/out
  syl-listing-pro-x e2e run --case single-listing-compliance-gate --tenant syl --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out --artifacts-id release-gate-20260311`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedCase := strings.TrimSpace(caseName)
			if resolvedCase == "" {
				resolvedCase = "release-gate"
			}
			if !isKnownE2ECase(resolvedCase) {
				return fmt.Errorf("未知 e2e 用例: %s", resolvedCase)
			}
			if strings.TrimSpace(tenant) == "" {
				return fmt.Errorf("缺少 tenant：请传 --tenant")
			}
			if strings.TrimSpace(outputDir) == "" {
				return fmt.Errorf("缺少输出目录：请传 --out")
			}
			if resolvedCase != "listing-compliance-gate" && strings.TrimSpace(inputPath) == "" {
				return fmt.Errorf("缺少输入文件：%s 需要传 --input", resolvedCase)
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
			svc := e2e.NewDefaultService(paths)
			result, err := svc.Run(cmd.Context(), e2e.RunInput{
				CaseName:       resolvedCase,
				Tenant:         tenant,
				SYLKey:         resolvedKey,
				AdminToken:     resolvedAdminToken,
				PrivateKeyPath: privateKeyPath,
				InputPath:      inputPath,
				OutputDir:      outputDir,
				WorkerURL:      workerURL,
				ArtifactsID:    artifactsID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), result.ArtifactsDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&caseName, "case", "release-gate", "用例名")
	cmd.Flags().StringVar(&tenant, "tenant", "", "租户 ID")
	cmd.Flags().StringVar(&key, "key", defaultKey, "SYL_LISTING_KEY，默认从 ~/.syl-listing-pro/.env 读取")
	cmd.Flags().StringVar(&adminToken, "admin-token", defaultAdminToken, "ADMIN_TOKEN，默认从 ~/.syl-listing-pro-x/.env 读取")
	cmd.Flags().StringVar(&privateKeyPath, "private-key", "", "规则签名私钥路径；不传则按环境变量解析")
	cmd.Flags().StringVar(&inputPath, "input", "", "输入 markdown 文件")
	cmd.Flags().StringVar(&outputDir, "out", "", "输出目录")
	cmd.Flags().StringVar(&workerURL, "worker", paths.WorkerURL, "worker 地址")
	cmd.Flags().StringVar(&artifactsID, "artifacts-id", "", "artifacts 目录名")
	return cmd
}

func isKnownE2ECase(name string) bool {
	switch strings.TrimSpace(name) {
	case "release-gate", "architecture-gate", "listing-compliance-gate", "single-listing-compliance-gate":
		return true
	default:
		return false
	}
}
