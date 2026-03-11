package rules

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	allowDevPrivateKeyEnv = "SYL_LISTING_ALLOW_DEV_PRIVATE_KEY"
	privateKeyPathEnv     = "SYL_LISTING_RULES_PRIVATE_KEY"
	privateKeyPEMEnv      = "SIGNING_PRIVATE_KEY_PEM"
	privateKeyBase64Env   = "SIGNING_PRIVATE_KEY_BASE64"
)

func (s Service) resolvePrivateKeyPath(explicit string) (string, func(), error) {
	if value := strings.TrimSpace(explicit); value != "" {
		return value, func() {}, nil
	}
	if value := strings.TrimSpace(os.Getenv(privateKeyPathEnv)); value != "" {
		return value, func() {}, nil
	}
	if value := os.Getenv(privateKeyPEMEnv); strings.TrimSpace(value) != "" {
		return writeTempPrivateKeyFile(value)
	}
	if value := strings.TrimSpace(os.Getenv(privateKeyBase64Env)); value != "" {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return "", func() {}, fmt.Errorf("%s 不是合法 base64: %w", privateKeyBase64Env, err)
		}
		return writeTempPrivateKeyFile(string(decoded))
	}
	if allowDevPrivateKey() {
		return filepath.Join(s.Root, "keys", "rules_private.pem"), func() {}, nil
	}
	return "", func() {}, fmt.Errorf("缺少私钥：先传 --private-key；如果不想显式传参，可依次设置 %s / %s / %s；只有本地开发模式才允许显式开启 %s=1 后回退到仓库内默认私钥", privateKeyPathEnv, privateKeyPEMEnv, privateKeyBase64Env, allowDevPrivateKeyEnv)
}

func writeTempPrivateKeyFile(content string) (string, func(), error) {
	file, err := os.CreateTemp("", "syl-rules-private-key-*.pem")
	if err != nil {
		return "", func() {}, err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	if err := os.Chmod(file.Name(), 0o600); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	return file.Name(), func() {
		_ = os.Remove(file.Name())
	}, nil
}

func (s Service) validatePrivateKeyPath(privateKeyPath string) error {
	devKeyPath := filepath.Join(s.Root, "keys", "rules_private.pem")
	samePath, err := sameFilePath(privateKeyPath, devKeyPath)
	if err != nil {
		return err
	}
	if samePath && !allowDevPrivateKey() {
		return fmt.Errorf("仓库内默认私钥仅允许本地开发模式使用；GitHub Actions / CI 必须通过 %s 或 %s 注入；如果当前只是本地调试，请显式设置 %s=1 后重试", privateKeyPEMEnv, privateKeyBase64Env, allowDevPrivateKeyEnv)
	}
	return nil
}

func allowDevPrivateKey() bool {
	if isCIEnvironment() {
		return false
	}
	raw := strings.TrimSpace(os.Getenv(allowDevPrivateKeyEnv))
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func isCIEnvironment() bool {
	for _, key := range []string{"GITHUB_ACTIONS", "CI"} {
		raw := strings.TrimSpace(os.Getenv(key))
		switch strings.ToLower(raw) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func sameFilePath(left, right string) (bool, error) {
	leftInfo, leftErr := os.Stat(left)
	if leftErr != nil && !os.IsNotExist(leftErr) {
		return false, leftErr
	}
	rightInfo, rightErr := os.Stat(right)
	if rightErr != nil && !os.IsNotExist(rightErr) {
		return false, rightErr
	}
	if leftInfo != nil && rightInfo != nil {
		return os.SameFile(leftInfo, rightInfo), nil
	}

	leftAbs, err := filepath.Abs(left)
	if err != nil {
		return false, err
	}
	rightAbs, err := filepath.Abs(right)
	if err != nil {
		return false, err
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs), nil
}
