package worker

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s Service) createWorkerArchive() (string, error) {
	repo := s.workerRepo()
	cfg, err := s.loadWorkerConfig()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.Server.Domain) == "" {
		return "", fmt.Errorf("config.server.domain 不能为空")
	}

	tmpFile, err := os.CreateTemp("", "syl-listing-worker-*.tar.gz")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer tmpFile.Close()

	gz := gzip.NewWriter(tmpFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	composeEnv := fmt.Sprintf("DOMAIN=%s\nLETSENCRYPT_EMAIL=%s\n", cfg.Server.Domain, cfg.Server.LetsencryptEmail)
	if err := addBytesToTar(tw, ".compose.env", []byte(composeEnv), 0o644); err != nil {
		return "", err
	}

	err = filepath.Walk(repo, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		if shouldSkipWorkerPath(name, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return "", err
	}
	return tmpPath, nil
}

func (s Service) loadWorkerConfig() (workerConfig, error) {
	var cfg workerConfig
	cfgData, err := os.ReadFile(filepath.Join(s.workerRepo(), "worker.config.json"))
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func shellSingleQuote(input string) string {
	return strings.ReplaceAll(input, "'", `'\''`)
}

func shellQuoteArg(input string) string {
	return "'" + shellSingleQuote(strings.TrimSpace(input)) + "'"
}

func shouldSkipWorkerPath(name string, isDir bool) bool {
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case ".git", "node_modules", "dist":
		return true
	case ".env":
		return true
	}
	if parts[0] == "data" {
		return true
	}
	if name == "data" && isDir {
		return true
	}
	return false
}

func addBytesToTar(tw *tar.Writer, name string, data []byte, mode int64) error {
	header := &tar.Header{
		Name:    name,
		Mode:    mode,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
