package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/config"
	"github.com/spf13/cobra"
)

var paths = config.DefaultPaths()

var rootCmd = &cobra.Command{
	Use:   "syl-listing-pro-x",
	Short: "syl-listing-pro-x 工程侧工具集合",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.AddCommand(newRulesCmd())
	rootCmd.AddCommand(newWorkerCmd())
	rootCmd.AddCommand(newE2ECmd())
}
