package cmd

import (
	"fmt"
	"io"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

func versionText() string {
	return fmt.Sprintf("syl-listing-pro-x 版本：%s（commit: %s，构建时间: %s）", Version, Commit, BuildTime)
}

func printVersion(w io.Writer) {
	fmt.Fprintln(w, versionText())
}
