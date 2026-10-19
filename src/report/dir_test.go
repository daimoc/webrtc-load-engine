package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDir(t *testing.T) {
	wd, _ := os.Getwd()
	t.Setenv("REPORTS_DIR", "")
	if got, _ := ResolveDir(""); got != filepath.Join(wd, "reports") {
		t.Errorf("default = %q", got)
	}
	t.Setenv("REPORTS_DIR", "env-reports")
	if got, _ := ResolveDir(""); got != filepath.Join(wd, "env-reports") {
		t.Errorf("from REPORTS_DIR = %q", got)
	}
	if got, _ := ResolveDir("/tmp/r009"); got != "/tmp/r009" {
		t.Errorf("from the flag = %q", got)
	}
	if got, _ := ResolveDir("rel"); got != filepath.Join(wd, "rel") {
		t.Errorf("relative flag = %q", got)
	}
}

func TestModuleRootDir(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644)
	start := filepath.Join(root, "tests", "live")
	os.MkdirAll(start, 0o755)
	if got, err := ModuleRootDir(start); err != nil || got != root {
		t.Errorf("ModuleRootDir = %q, %v", got, err)
	}
	if _, err := ModuleRootDir(t.TempDir()); err == nil {
		t.Error("no error without go.mod")
	}
}
