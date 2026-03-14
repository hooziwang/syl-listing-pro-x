package cmd

import (
	"fmt"
	"io"
)

type rulesCommandPathContext struct {
	WorkspaceRoot string
	RulesRepo     string
	PackageDir    string
	RulesVersion  string
}

func writeRulesCommandPathContext(w io.Writer, action string, ctx rulesCommandPathContext) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "[rules %s] 路径上下文\n", action)
	fmt.Fprintf(w, "WorkspaceRoot=%s\n", ctx.WorkspaceRoot)
	fmt.Fprintf(w, "RulesRepo=%s\n", ctx.RulesRepo)
	if ctx.PackageDir != "" {
		fmt.Fprintf(w, "PackageDir=%s\n", ctx.PackageDir)
	}
	if ctx.RulesVersion != "" {
		fmt.Fprintf(w, "RulesVersion=%s\n", ctx.RulesVersion)
	}
}
