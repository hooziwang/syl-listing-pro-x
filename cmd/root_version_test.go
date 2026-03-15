package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandVersionFlagPrintsVersion(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "syl-listing-pro-x 版本：") {
		t.Fatalf("version output missing: %q", out.String())
	}
}

func TestVersionSubcommandPrintsVersion(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "syl-listing-pro-x 版本：") {
		t.Fatalf("version subcommand output missing: %q", out.String())
	}
}
