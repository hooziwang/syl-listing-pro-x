package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateVerboseExecution(t *testing.T) {
	t.Run("stderr non-empty fails", func(t *testing.T) {
		logPath := writeVerboseLog(t, `{"event":"cli_start","level":"info"}`)
		err := validateVerboseExecution([]byte("boom\n"), logPath)
		if err == nil {
			t.Fatal("expected stderr validation error")
		}
		if !strings.Contains(err.Error(), "stderr") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid ndjson line fails", func(t *testing.T) {
		logPath := writeVerboseLog(t, `{"event":"cli_start","level":"info"}`, `not-json`)
		err := validateVerboseExecution(nil, logPath)
		if err == nil {
			t.Fatal("expected invalid ndjson error")
		}
		if !strings.Contains(err.Error(), "NDJSON") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error level fails", func(t *testing.T) {
		logPath := writeVerboseLog(t, `{"event":"cli_start","level":"error"}`)
		err := validateVerboseExecution(nil, logPath)
		if err == nil {
			t.Fatal("expected error level failure")
		}
		if !strings.Contains(err.Error(), "level=error") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("failed event fails", func(t *testing.T) {
		logPath := writeVerboseLog(t, `{"event":"job_failed","level":"info"}`)
		err := validateVerboseExecution(nil, logPath)
		if err == nil {
			t.Fatal("expected failed event failure")
		}
		if !strings.Contains(err.Error(), "job_failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("failed status fails", func(t *testing.T) {
		logPath := writeVerboseLog(t, `{"event":"job_finished","status":"failed"}`)
		err := validateVerboseExecution(nil, logPath)
		if err == nil {
			t.Fatal("expected failed status failure")
		}
		if !strings.Contains(err.Error(), "status=failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("info events pass", func(t *testing.T) {
		logPath := writeVerboseLog(t,
			`{"event":"cli_start","level":"info"}`,
			`{"event":"job_running","status":"running"}`,
			`{"event":"job_succeeded","level":"info"}`,
		)
		if err := validateVerboseExecution(nil, logPath); err != nil {
			t.Fatalf("validateVerboseExecution() error = %v", err)
		}
	})

	t.Run("empty log fails", func(t *testing.T) {
		logPath := writeVerboseLog(t)
		err := validateVerboseExecution(nil, logPath)
		if err == nil {
			t.Fatal("expected empty log failure")
		}
		if !strings.Contains(err.Error(), "verbose") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func writeVerboseLog(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.verbose.ndjson")
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
