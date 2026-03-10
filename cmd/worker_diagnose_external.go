package cmd

import (
	"time"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerDiagnoseExternalCmd() *cobra.Command {
	var baseURL string
	var sylKey string
	var withGenerate bool
	var timeout time.Duration
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "diagnose-external",
		Short: "对外诊断 worker 接口",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{}
			return svc.DiagnoseExternal(cmd.Context(), worker.DiagnoseExternalInput{
				BaseURL:      baseURL,
				SYLKey:       sylKey,
				WithGenerate: withGenerate,
				Timeout:      timeout,
				Interval:     interval,
			})
		},
	}
	cmd.Flags().StringVar(&baseURL, "base-url", "https://worker.aelus.tech", "worker 对外地址")
	cmd.Flags().StringVar(&sylKey, "key", "", "SYL_LISTING_KEY")
	cmd.Flags().BoolVar(&withGenerate, "with-generate", false, "额外执行一次生成链路检查")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "生成轮询超时")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "轮询间隔")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}
