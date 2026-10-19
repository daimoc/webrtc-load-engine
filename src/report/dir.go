package report

import (
	"errors"
	"os"
	"path/filepath"
)

// EnvDir names the environment variable that sets the reports folder
// (contracts/configuration.md).
const EnvDir = "REPORTS_DIR"

// ResolveDir returns the absolute reports folder: flagValue when set, else
// $REPORTS_DIR, else "reports" in the working directory.
func ResolveDir(flagValue string) (string, error) {
	dir := flagValue
	if dir == "" {
		dir = os.Getenv(EnvDir)
	}
	if dir == "" {
		dir = "reports"
	}
	return filepath.Abs(dir)
}

// ModuleRootDir is the closest folder at or above start holding go.mod.
// The live tests use it: go test runs them in their package folder.
func ModuleRootDir(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod at or above " + start)
		}
		dir = parent
	}
}
