package cmd

import "path/filepath"

func filepathOrDefault(root, rel string) string {
	return filepath.Join(root, rel)
}
