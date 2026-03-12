package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func validateVerboseExecution(stderr []byte, logPath string) error {
	if len(bytes.TrimSpace(stderr)) > 0 {
		return fmt.Errorf("CLI stderr 非空: %s", strings.TrimSpace(string(stderr)))
	}

	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("读取 verbose 日志失败: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	eventCount := 0
	for scanner.Scan() {
		lineNo++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		var event map[string]any
		if err := json.Unmarshal([]byte(text), &event); err != nil {
			return fmt.Errorf("CLI verbose NDJSON 第 %d 行不是合法 JSON", lineNo)
		}
		eventCount++

		level := strings.ToLower(strings.TrimSpace(stringValue(event["level"])))
		if level == "error" {
			return fmt.Errorf("CLI verbose 出现 level=error，第 %d 行", lineNo)
		}

		name := strings.ToLower(strings.TrimSpace(stringValue(event["event"])))
		if strings.Contains(name, "failed") || strings.Contains(name, "error") {
			return fmt.Errorf("CLI verbose 出现错误事件 %q，第 %d 行", stringValue(event["event"]), lineNo)
		}

		status := strings.ToLower(strings.TrimSpace(stringValue(event["status"])))
		if status == "failed" {
			return fmt.Errorf("CLI verbose 出现 status=failed，第 %d 行", lineNo)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 verbose 日志失败: %w", err)
	}
	if eventCount == 0 {
		return fmt.Errorf("CLI verbose 日志为空")
	}

	return nil
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
