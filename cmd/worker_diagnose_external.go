package cmd

import (
	"time"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerDiagnoseExternalCmd() *cobra.Command {
	var baseURL string
	var sylKey string
	var expectedTenant string
	var withGenerate bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "diagnose-external",
		Short: "对外诊断 worker 接口",
		Long: `从外部对 worker HTTP 接口做黑盒诊断。

默认会检查 /healthz、/v1/auth/exchange、/v1/rules/resolve、/v1/rules/refresh 和规则下载链路；如果传 --with-generate，还会额外发起一次真实生成并订阅 SSE 任务事件。
这个命令不会改远端代码，但会访问真实 worker 服务和真实租户规则。`,
		Example: `  syl-listing-pro-x worker diagnose-external --key <SYL_LISTING_KEY>
  syl-listing-pro-x worker diagnose-external --base-url https://worker.example.test --key <SYL_LISTING_KEY>
  syl-listing-pro-x worker diagnose-external --key <SYL_LISTING_KEY> --with-generate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{}
			return svc.DiagnoseExternal(cmd.Context(), worker.DiagnoseExternalInput{
				BaseURL:        baseURL,
				SYLKey:         sylKey,
				ExpectedTenant: expectedTenant,
				WithGenerate:   withGenerate,
				Timeout:        timeout,
			})
		},
	}
	cmd.Flags().StringVar(&baseURL, "base-url", paths.WorkerURL, "worker 对外地址")
	cmd.Flags().StringVar(&sylKey, "key", "", "SYL_LISTING_KEY")
	cmd.Flags().StringVar(&expectedTenant, "expected-tenant", "", "期望命中的租户 ID")
	cmd.Flags().BoolVar(&withGenerate, "with-generate", false, "额外执行一次生成链路检查")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "生成事件流超时")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}
