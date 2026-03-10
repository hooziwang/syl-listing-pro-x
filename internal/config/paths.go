package config

import "path/filepath"

type Paths struct {
	WorkspaceRoot string
	WorkerRepo    string
	RulesRepo     string
}

func DefaultPaths() Paths {
	root := "/Users/wxy/syl-listing-pro"
	return Paths{
		WorkspaceRoot: root,
		WorkerRepo:    filepath.Join(root, "worker"),
		RulesRepo:     filepath.Join(root, "rules"),
	}
}
