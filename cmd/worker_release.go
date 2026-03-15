package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerReleaseCmd() *cobra.Command {
	var server string
	var version string
	var skipDiagnose bool
	var skipBuild bool
	var stopLegacy bool
	var installDocker bool
	var skipWaitHTTPS bool
	var httpsTimeout int
	var httpsCheckInterval int
	cmd := &cobra.Command{
		Use:   "release",
		Short: "发布 worker 到远端服务器",
		Long: `worker release 是唯一正式发布入口。

它会先校验本地 worker 仓工作区干净并执行 npm test，再创建并推送版本 tag，随后从该 tag 对应代码创建临时发布副本并部署到远端，最后核对远端运行版本。

必须显式传入 --server。
必须显式传入 --version。
只允许从 tag 对应代码发布。`,
		Example: `  syl-listing-pro-x worker release --server syl-server --version v0.1.3
  syl-listing-pro-x worker release --server syl-server --version v0.1.3 --skip-build`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{
				WorkerRepo: paths.WorkerRepo,
				Servers:    worker.DefaultServers(),
			}
			return svc.Release(cmd.Context(), worker.ReleaseInput{
				Server:             server,
				Version:            version,
				SkipBuild:          skipBuild,
				StopLegacy:         stopLegacy,
				InstallDocker:      installDocker,
				SkipWaitHTTPS:      skipWaitHTTPS,
				HTTPSTimeout:       httpsTimeout,
				HTTPSCheckInterval: httpsCheckInterval,
				SkipDiagnose:       skipDiagnose,
			})
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "服务器别名")
	_ = cmd.MarkFlagRequired("server")
	cmd.Flags().StringVar(&version, "version", "", "worker 版本 tag，例如 v0.1.3")
	_ = cmd.MarkFlagRequired("version")
	cmd.Flags().BoolVar(&skipBuild, "skip-build", false, "跳过镜像构建")
	cmd.Flags().BoolVar(&stopLegacy, "stop-legacy", false, "停止旧 systemd 服务")
	cmd.Flags().BoolVar(&installDocker, "install-docker", false, "缺少 docker 时自动安装")
	cmd.Flags().BoolVar(&skipWaitHTTPS, "skip-wait-https", false, "跳过 HTTPS 就绪等待")
	cmd.Flags().IntVar(&httpsTimeout, "https-timeout", 240, "HTTPS 等待超时秒数")
	cmd.Flags().IntVar(&httpsCheckInterval, "https-interval", 2, "HTTPS 就绪检查间隔秒数")
	cmd.Flags().BoolVar(&skipDiagnose, "skip-diagnose", false, "发布后跳过部署诊断")
	return cmd
}
