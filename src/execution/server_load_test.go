package execution

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"webrtc-load-engine/src/scenario"
)

// fakeDockerSocket serves the stats of one running container, "jvb", on a
// Unix socket, and 404 for any other name; it sets DOCKER_HOST to it.
func fakeDockerSocket(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "engine-docker")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "docker.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/containers/jvb/") {
			http.NotFound(w, r)
			return
		}
		// Half a thread busy since start.
		cpu := time.Since(start).Nanoseconds() / 2
		w.Write([]byte(`{"read":"2026-10-17T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":` +
			strconv.FormatInt(cpu, 10) + `}},"memory_stats":{"usage":1048576,"stats":{"inactive_file":0}}}`))
	}))
	s.Listener = l
	s.Start()
	t.Cleanup(s.Close)
	t.Setenv("DOCKER_HOST", "unix://"+socket)
}

// runServerLoadScenario runs 2 bots for 2 s and returns the status, the
// summary and how long the run took.
func runServerLoadScenario(t *testing.T, monitoring *scenario.ServerMonitoringConfig) (string, *scenario.QualitySummary, time.Duration) {
	t.Helper()
	plan := scenario.TestPlan{Participants: 2, DurationSeconds: 2, ServerMonitoring: monitoring}
	done := make(chan struct{})
	var status string
	var summary *scenario.QualitySummary
	engine := NewEngine("engine-load-"+uuid.NewString(), plan, &MockPlatformAdapter{t: t}, quietLogger,
		func(_ string, r scenario.RunResult) {
			s, q := r.Status, r.Quality
			status, summary = s, q
			close(done)
		})
	start := time.Now()
	go engine.Start()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("scenario did not complete")
	}
	return status, summary, time.Since(start)
}

func TestEngine_ServerLoad(t *testing.T) {
	fakeDockerSocket(t)
	_, _, plain := runServerLoadScenario(t, nil)
	status, summary, took := runServerLoadScenario(t, &scenario.ServerMonitoringConfig{
		Containers: []string{"jvb", "missing"}, IntervalSeconds: 1,
	})
	if status != "completed" {
		t.Fatalf("status %q, want completed", status)
	}
	if took > plain+time.Second {
		t.Errorf("run took %v with monitoring, %v without", took, plain)
	}
	if summary == nil {
		t.Fatal("no summary")
	}
	server := summary.Server
	if !server.Available || len(server.Containers) != 2 {
		t.Fatalf("server %+v", server)
	}
	jvb, missing := server.Containers[0], server.Containers[1]
	if jvb.Status != scenario.LoadStatusMeasured || jvb.Samples < 3 || jvb.CPUThreads.Average < 0.4 || jvb.CPUThreads.Average > 0.6 {
		t.Errorf("jvb %+v, want measured at about half a thread", jvb)
	}
	if missing.Status != scenario.LoadStatusUnavailable || missing.Reason != scenario.ReasonNotFound {
		t.Errorf("missing %+v", missing)
	}
	m := summary.LoadMachine
	if m.Period == nil || *m.Period != *server.Period || m.Engine.Samples != jvb.Samples || m.Engine.MemoryBytes.Peak <= 0 || m.HardwareThreads < 1 {
		t.Errorf("load machine %+v (server period %+v)", m, server.Period)
	}
}

func TestEngine_ServerLoadNotRequested(t *testing.T) {
	_, summary, _ := runServerLoadScenario(t, nil)
	if summary == nil || summary.Server.Available || summary.Server.Reason != scenario.ReasonNotRequested {
		t.Errorf("summary server %+v, want not_requested", summary.Server)
	}
	// The engine is sampled without containers too: at start and at the end.
	if m := summary.LoadMachine; m.Engine.Samples < 2 || m.Engine.MemoryBytes.Peak <= 0 {
		t.Errorf("load machine %+v", m)
	}
}
