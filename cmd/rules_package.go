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
	var printPathContext bool
	cmd := &cobra.Command{
		Use:   "package",
		Short: "打包租户规则",
		Long: `先执行规则校验，再生成 rules.tar.gz、manifest.json 和公钥文件。

产物会写入 rules/dist/<tenant>/<version>/，stdout 输出产物目录路径。传 --print-path-context 时，stderr 打印本次解析到的路径上下文。
签名私钥优先读取 --private-key，其次依次读取 SYL_LISTING_RULES_PRIVATE_KEY、SIGNING_PRIVATE_KEY_PEM、SIGNING_PRIVATE_KEY_BASE64；都没有时，只有显式开启本地开发模式才允许回退到仓库内默认私钥。`,
		Example: `  syl-listing-pro-x rules package --tenant syl
  syl-listing-pro-x rules package --tenant syl --version rules-syl-20260311-120000-ab12cd
  syl-listing-pro-x rules package --tenant syl --private-key /abs/rules.pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := rules.Service{Root: paths.RulesRepo}
			if version == "" {
				version = rules.GenerateVersion(tenant)
			}
			out, err := svc.Package(tenant, version, privateKey)
			if err != nil {
				return err
			}
			if printPathContext {
				writeRulesCommandPathContext(cmd.ErrOrStderr(), "package", rulesCommandPathContext{
					WorkspaceRoot: paths.WorkspaceRoot,
					RulesRepo:     paths.RulesRepo,
					PackageDir:    out.PackageDir,
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), out.PackageDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "租户 ID")
	cmd.Flags().StringVar(&version, "version", "", "规则版本，不传则自动生成")
	cmd.Flags().StringVar(&privateKey, "private-key", "", "签名私钥路径；不传则按环境变量或开发模式解析")
	cmd.Flags().BoolVar(&printPathContext, "print-path-context", false, "把本次解析到的路径上下文打印到 stderr")
	_ = cmd.MarkFlagRequired("tenant")
	return cmd
}
