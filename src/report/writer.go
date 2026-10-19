package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"webrtc-load-engine/src/scenario"
)

// maxSuffix bounds the "-2", "-3"... suffixes tried when a name is taken.
const maxSuffix = 1000

// Writer writes run reports into Dir.
type Writer struct {
	Dir    string       // Absolute folder, created if missing
	Logger *slog.Logger // nil: slog.Default()
}

var _ scenario.Reporter = Writer{}

// WriteReport implements scenario.Reporter: it writes the report of the run
// r of the redacted scenario sc.
func (w Writer) WriteReport(sc *scenario.Scenario, r scenario.RunResult) scenario.ReportFiles {
	return w.Write(NewRun(sc, r))
}

// marshalJSON encodes the JSON report (a variable so tests can make it
// fail).
var marshalJSON = json.MarshalIndent

// Write writes the report files of run and says where they went: the JSON
// file, then the Markdown file with the same base name. It never fails: a
// file that cannot be written has its Error set and is logged once, and
// does not stop the other (FR-012).
func (w Writer) Write(run Run) scenario.ReportFiles {
	logger := w.Logger
	if logger == nil {
		logger = slog.Default()
	}
	files := scenario.ReportFiles{RunID: run.RunID}
	base := baseName(run.EndedAt, run.Scenario.Platform, run.Scenario.Name, run.Scenario.ID, run.RunID)
	mkdirErr := os.MkdirAll(w.Dir, 0o755)

	write := func(kind, ext string, data []byte, err error) scenario.ReportFile {
		if err == nil {
			err = mkdirErr
		}
		if err == nil {
			var path string
			// The JSON file picks the base; the Markdown file follows it.
			if path, base, err = publish(w.Dir, base, ext, data); err == nil {
				return scenario.ReportFile{Path: path}
			}
		}
		logger.Warn("Run report not written", slog.String("scenario_id", run.Scenario.ID),
			slog.String("run_id", run.RunID), slog.String("file", kind), slog.String("error", err.Error()))
		return scenario.ReportFile{Error: err.Error()}
	}
	data, err := marshalJSON(run, "", "  ")
	files.JSON = write("json", ".json", append(data, '\n'), err)
	files.Markdown = write("markdown", ".md", Markdown(run), nil)

	if files.JSON.Path != "" || files.Markdown.Path != "" {
		logger.Info("Run report written", slog.String("scenario_id", run.Scenario.ID), slog.String("run_id", run.RunID),
			slog.String("json", files.JSON.Path), slog.String("markdown", files.Markdown.Path))
	}
	return files
}

// publish writes data to <dir>/<base><ext> without ever overwriting a file
// and without a reader seeing it half written (research R3): the data goes
// to a temporary file which is then linked to the final name. When the name
// is taken, "-2", "-3"... are appended to base. It returns the path and the
// base used.
func publish(dir, base, ext string, data []byte) (string, string, error) {
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return "", base, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}
	if err != nil {
		return "", base, err
	}
	for i := 1; i <= maxSuffix; i++ {
		b := base
		if i > 1 {
			b = fmt.Sprintf("%s-%d", base, i)
		}
		path := filepath.Join(dir, b+ext)
		err := os.Link(tmp.Name(), path)
		if err == nil {
			return path, b, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", base, err
		}
	}
	return "", base, fmt.Errorf("%s%s: %d names taken", filepath.Join(dir, base), ext, maxSuffix)
}
