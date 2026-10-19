package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

func TestWriteJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "reports") // Missing, nested
	run := NewRun(testScenario(), testResult())
	files := Writer{Dir: dir}.Write(run)
	if files.RunID != run.RunID {
		t.Errorf("run ID %q, want %q", files.RunID, run.RunID)
	}
	if files.JSON.Error != "" {
		t.Fatalf("JSON error: %s", files.JSON.Error)
	}
	want := filepath.Join(dir, baseName(run.EndedAt, "jitsi", run.Scenario.Name, run.Scenario.ID, run.RunID)+".json")
	if files.JSON.Path != want {
		t.Errorf("path %q, want %q", files.JSON.Path, want)
	}
	info, err := os.Stat(files.JSON.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(files.JSON.Path)
	var back Run
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.RunID != run.RunID || back.Quality.Bots != 10 || back.Scenario.Name != run.Scenario.Name {
		t.Errorf("read back %+v", back)
	}
	assertNoTemp(t, dir)
}

func TestWriteNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	run := NewRun(testScenario(), testResult())
	base := baseName(run.EndedAt, "jitsi", run.Scenario.Name, run.Scenario.ID, run.RunID)
	// A file already holding the first name is kept as it is.
	existing := filepath.Join(dir, base+".json")
	os.WriteFile(existing, []byte("earlier"), 0o644)
	paths := map[string]bool{}
	for i := 0; i < 50; i++ {
		files := Writer{Dir: dir}.Write(run) // Same end time, name and run ID
		if files.JSON.Error != "" {
			t.Fatalf("write %d: %s", i, files.JSON.Error)
		}
		if paths[files.JSON.Path] || files.JSON.Path == existing {
			t.Fatalf("write %d reused %s", i, files.JSON.Path)
		}
		paths[files.JSON.Path] = true
	}
	if raw, _ := os.ReadFile(existing); string(raw) != "earlier" {
		t.Errorf("existing file overwritten: %q", raw)
	}
	if !paths[filepath.Join(dir, base+"-2.json")] || !paths[filepath.Join(dir, base+"-51.json")] {
		t.Errorf("suffixes: %v", paths)
	}
	assertNoTemp(t, dir)
}

func TestWriteUnwritableFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	parent := t.TempDir()
	os.Chmod(parent, 0o555)
	t.Cleanup(func() { os.Chmod(parent, 0o755) })
	var logs bytes.Buffer
	files := Writer{Dir: filepath.Join(parent, "reports"), Logger: slog.New(slog.NewTextHandler(&logs, nil))}.Write(NewRun(testScenario(), testResult()))
	if files.JSON.Path != "" || !strings.Contains(files.JSON.Error, "permission denied") {
		t.Errorf("JSON = %+v", files.JSON)
	}
	out := logs.String()
	if strings.Count(out, "level=WARN") < 1 || !strings.Contains(out, "scenario_id=scenario-6d0c1e2a") || !strings.Contains(out, "run_id=") {
		t.Errorf("logs: %s", out)
	}
}

func TestWriteNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	run := NewRun(testScenario(), testResult())
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // A reader sees either no report or a complete one.
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
			for _, m := range matches {
				raw, err := os.ReadFile(m)
				if err == nil && !json.Valid(raw) {
					t.Errorf("partial file %s: %d bytes", m, len(raw))
				}
			}
			time.Sleep(100 * time.Microsecond)
		}
	}()
	for i := 0; i < 20; i++ {
		Writer{Dir: dir}.Write(run)
	}
	close(stop)
	wg.Wait()
}

// assertNoTemp fails if a temporary file is left in dir.
func assertNoTemp(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temporary file left: %s", e.Name())
		}
	}
}

func TestWriteBothFiles(t *testing.T) {
	dir := t.TempDir()
	run := NewRun(testScenario(), testResult())
	files := Writer{Dir: dir}.Write(run)
	if files.JSON.Error != "" || files.Markdown.Error != "" {
		t.Fatalf("errors: %+v", files)
	}
	if strings.TrimSuffix(files.JSON.Path, ".json") != strings.TrimSuffix(files.Markdown.Path, ".md") {
		t.Errorf("bases differ: %s, %s", files.JSON.Path, files.Markdown.Path)
	}
	md, _ := os.ReadFile(files.Markdown.Path)
	if !bytes.Equal(md, Markdown(run)) {
		t.Error("Markdown file differs from Markdown(run)")
	}
}

func TestWritePairsNeverOverwrite(t *testing.T) {
	dir := t.TempDir()
	run := NewRun(testScenario(), testResult())
	for i := 0; i < 50; i++ {
		files := Writer{Dir: dir}.Write(run)
		if files.JSON.Error != "" || files.Markdown.Error != "" {
			t.Fatalf("write %d: %+v", i, files)
		}
		if strings.TrimSuffix(files.JSON.Path, ".json") != strings.TrimSuffix(files.Markdown.Path, ".md") {
			t.Fatalf("write %d: bases differ: %+v", i, files)
		}
	}
	jsons, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	mds, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	if len(jsons) != 50 || len(mds) != 50 {
		t.Errorf("%d JSON and %d Markdown files, want 50 pairs", len(jsons), len(mds))
	}
}

func TestWriteUnwritableFolderBothFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	parent := t.TempDir()
	os.Chmod(parent, 0o555)
	t.Cleanup(func() { os.Chmod(parent, 0o755) })
	files := Writer{Dir: filepath.Join(parent, "reports"), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}.Write(NewRun(testScenario(), testResult()))
	if files.JSON.Error == "" || files.Markdown.Error == "" || files.JSON.Path != "" || files.Markdown.Path != "" {
		t.Errorf("files %+v", files)
	}
}

// One file failing does not stop the other (spec edge case).
func TestWriteJSONFailureKeepsMarkdown(t *testing.T) {
	marshal := marshalJSON
	marshalJSON = func(any, string, string) ([]byte, error) { return nil, errors.New("encoding failed") }
	t.Cleanup(func() { marshalJSON = marshal })
	dir := t.TempDir()
	files := Writer{Dir: dir, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}.Write(NewRun(testScenario(), testResult()))
	if files.JSON.Error != "encoding failed" || files.Markdown.Path == "" {
		t.Errorf("files %+v", files)
	}
}

// No secret of a test plan reaches a report file (SC-006).
func TestWriteNoSecrets(t *testing.T) {
	dir := t.TempDir()
	keys := map[string][]string{"jitsi": {"jwt_app_secret", "jwt"}, "livekit": {"api_secret", "token"}}
	for platform, config := range map[string]map[string]any{
		"jitsi":   {"xmpp_domain": "meet.jitsi", "jwt_app_id": "app", "jwt_app_secret": "s3cr3t-009", "jwt": "tok-009"},
		"livekit": {"url": "ws://localhost:7880", "api_key": "devkey", "api_secret": "s3cr3t-lk", "token": "tok-lk"},
	} {
		sc := testScenario()
		sc.Platform = platform
		sc.TestPlan = scenario.RedactTestPlan(scenario.TestPlan{Participants: 2, DurationSeconds: 10, PlatformConfig: config}, keys[platform])
		Writer{Dir: dir}.WriteReport(sc, testResult())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 4 {
		t.Fatalf("%d files, want 4", len(entries))
	}
	for _, e := range entries {
		raw, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		for _, secret := range []string{"s3cr3t-009", "tok-009", "s3cr3t-lk", "tok-lk"} {
			if bytes.Contains(raw, []byte(secret)) {
				t.Errorf("%s contains %q", e.Name(), secret)
			}
		}
		if !bytes.Contains(raw, []byte(scenario.Redacted)) {
			t.Errorf("%s has no redacted value", e.Name())
		}
	}
}
