package e2e

import (
	"fmt"
	"io"
)

type e2eRulesPathContext struct {
	RulesRepo    string
	PackageDir   string
	RulesVersion string
}

func writeE2ERulesPathContext(w io.Writer, ctx e2eRulesPathContext) {
	if w == nil {
		return
	}
	fmt.Fprintln(w, "[e2e rules publish] 路径上下文")
	fmt.Fprintf(w, "RulesRepo=%s\n", ctx.RulesRepo)
	fmt.Fprintf(w, "PackageDir=%s\n", ctx.PackageDir)
	fmt.Fprintf(w, "RulesVersion=%s\n", ctx.RulesVersion)
}
