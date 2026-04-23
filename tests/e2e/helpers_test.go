package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// projectRoot returns the absolute path to the project root directory.
// It walks up from the current working directory looking for go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// projectPath returns an absolute path to a file relative to the project root.
func projectPath(t *testing.T, relPath string) string {
	t.Helper()
	return filepath.Join(projectRoot(t), relPath)
}
