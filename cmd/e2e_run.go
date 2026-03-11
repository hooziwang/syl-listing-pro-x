package cmd

import (
	"fmt"

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
	cmd := &cobra.Command{
		Use:   "run",
		Short: "执行 e2e 用例",
		Long: `执行真实发布验收。会先发布规则，再诊断 worker，最后调用 syl-listing-pro。

这个命令依赖 PATH 中可执行的 syl-listing-pro，并会在运行时把 CLI stdout、stderr 和 verbose 日志写入 artifacts 目录。stdout 只打印 artifacts 目录路径。
可用用例：release-gate, architecture-gate。`,
		Example: `  syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e run --case architecture-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --private-key /abs/rules.pem --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out --artifacts-id release-gate-20260311`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := e2e.NewDefaultService(paths)
			result, err := svc.Run(cmd.Context(), e2e.RunInput{
				CaseName:       caseName,
				Tenant:         tenant,
				SYLKey:         key,
				AdminToken:     adminToken,
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
	cmd.Flags().StringVar(&key, "key", "", "SYL_LISTING_KEY")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "ADMIN_TOKEN")
	cmd.Flags().StringVar(&privateKeyPath, "private-key", "", "规则签名私钥路径；不传则按环境变量解析")
	cmd.Flags().StringVar(&inputPath, "input", "", "输入 markdown 文件")
	cmd.Flags().StringVar(&outputDir, "out", "", "输出目录")
	cmd.Flags().StringVar(&workerURL, "worker", paths.WorkerURL, "worker 地址")
	cmd.Flags().StringVar(&artifactsID, "artifacts-id", "", "artifacts 目录名")
	_ = cmd.MarkFlagRequired("tenant")
	_ = cmd.MarkFlagRequired("key")
	_ = cmd.MarkFlagRequired("admin-token")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}
