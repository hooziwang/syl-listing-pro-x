package rules

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func (s Service) Package(tenant, version, privateKeyPath string) (PackageOutput, error) {
	if err := s.Validate(tenant); err != nil {
		return PackageOutput{}, err
	}
	resolvedPrivateKeyPath, cleanup, err := s.resolvePrivateKeyPath(privateKeyPath)
	if err != nil {
		return PackageOutput{}, err
	}
	defer cleanup()
	if err := s.validatePrivateKeyPath(resolvedPrivateKeyPath); err != nil {
		return PackageOutput{}, err
	}
	if _, err := os.Stat(resolvedPrivateKeyPath); err != nil {
		return PackageOutput{}, fmt.Errorf("签名私钥不存在: %s", resolvedPrivateKeyPath)
	}

	src := filepath.Join(s.Root, "tenants", tenant)
	outDir := filepath.Join(s.Root, "dist", tenant, version)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return PackageOutput{}, err
	}

	publicKeyPath := filepath.Join(outDir, "rules_signing_public.pem")
	if err := runOpenSSL("pkey", "-in", resolvedPrivateKeyPath, "-pubout", "-out", publicKeyPath); err != nil {
		return PackageOutput{}, err
	}

	archivePath := filepath.Join(outDir, "rules.tar.gz")
	if err := createTarGz(archivePath, map[string]string{
		src:           "tenant",
		publicKeyPath: "tenant/rules_signing_public.pem",
	}); err != nil {
		return PackageOutput{}, err
	}

	archiveSHA, err := sha256File(archivePath)
	if err != nil {
		return PackageOutput{}, err
	}
	signature, err := signFile(resolvedPrivateKeyPath, archivePath)
	if err != nil {
		return PackageOutput{}, err
	}
	publicKeySig, err := signFile(resolvedPrivateKeyPath, publicKeyPath)
	if err != nil {
		return PackageOutput{}, err
	}

	manifest := Manifest{
		TenantID:                        tenant,
		RulesVersion:                    version,
		ManifestSHA256:                  archiveSHA,
		SignatureBase64:                 signature,
		SignatureAlgo:                   "rsa-sha256",
		SigningPublicKeyPathInArchive:   "tenant/rules_signing_public.pem",
		SigningPublicKeySignatureBase64: publicKeySig,
		SigningPublicKeySignatureAlgo:   "rsa-sha256",
		Archive:                         "rules.tar.gz",
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return PackageOutput{}, err
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return PackageOutput{}, err
	}

	return PackageOutput{
		PackageDir:   outDir,
		ArchivePath:  archivePath,
		ManifestPath: manifestPath,
	}, nil
}

func createTarGz(outPath string, mappings map[string]string) error {
	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	keys := make([]string, 0, len(mappings))
	for src := range mappings {
		keys = append(keys, src)
	}
	sort.Strings(keys)

	for _, src := range keys {
		dst := mappings[src]
		if err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relPath := dst
			if path != src {
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				relPath = filepath.ToSlash(filepath.Join(dst, rel))
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = relPath
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
		}); err != nil {
			return err
		}
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func signFile(privateKeyPath, path string) (string, error) {
	cmd := exec.Command("openssl", "dgst", "-sha256", "-sign", privateKeyPath, path)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func runOpenSSL(args ...string) error {
	cmd := exec.Command("openssl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
