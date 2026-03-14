package cmd

import (
	"context"
	"io"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/e2e"
)

type e2eRunner interface {
	ListCases() []string
	Run(ctx context.Context, in e2e.RunInput) (e2e.RunResult, error)
}

var newE2ERunner = func(stderr io.Writer) e2eRunner {
	return e2e.NewDefaultService(paths, stderr)
}
