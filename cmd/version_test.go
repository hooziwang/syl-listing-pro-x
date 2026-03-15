package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionAndPrint(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	oldBuildTime := BuildTime
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		BuildTime = oldBuildTime
	})

	Version = "1.2.3"
	Commit = "abc1234"
	BuildTime = "2026-03-15T08:00:00Z"

	text := versionText()
	if !strings.Contains(text, "syl-listing-pro-x 版本：1.2.3") {
		t.Fatalf("versionText() missing version: %s", text)
	}
	if !strings.Contains(text, "commit: abc1234") {
		t.Fatalf("versionText() missing commit: %s", text)
	}
	if !strings.Contains(text, "构建时间: 2026-03-15T08:00:00Z") {
		t.Fatalf("versionText() missing build time: %s", text)
	}

	var buf bytes.Buffer
	printVersion(&buf)
	if got := buf.String(); !strings.Contains(got, text) {
		t.Fatalf("printVersion() output = %q, want contain %q", got, text)
	}
}
