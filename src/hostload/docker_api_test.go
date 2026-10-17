package hostload

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

// fakeDocker serves a Docker Engine API handler on a Unix socket in a
// temporary directory.
type fakeDocker struct {
	socket string
	server *httptest.Server
}

func newFakeDocker(t *testing.T, handler http.HandlerFunc) *fakeDocker {
	t.Helper()
	// Unix socket paths are limited to about 100 bytes: t.TempDir() can be longer.
	dir, err := os.MkdirTemp("", "hostload")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "docker.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewUnstartedServer(handler)
	s.Listener = l
	s.Start()
	t.Cleanup(s.Close)
	return &fakeDocker{socket: socket, server: s}
}

// runningStats is a running container's stats.
func runningStats(cpuNS uint64, mem int64) string {
	return `{"read":"2026-10-16T21:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":` +
		itoa(int64(cpuNS)) + `}},"memory_stats":{"usage":` + itoa(mem) + `,"stats":{"inactive_file":0}}}`
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// containersHandler answers stats for the given bodies and 404 for other
// names, as Docker does.
func containersHandler(bodies map[string]func() string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// /containers/{name}/stats
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "containers" || parts[2] != "stats" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("stream") != "false" || r.URL.Query().Get("one-shot") != "true" {
			http.Error(w, "want stream=false&one-shot=true", http.StatusBadRequest)
			return
		}
		body, ok := bodies[parts[1]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"No such container: ` + parts[1] + `"}`))
			return
		}
		w.Write([]byte(body()))
	}
}

func TestDockerClient_Stats(t *testing.T) {
	fake := newFakeDocker(t, containersHandler(map[string]func() string{
		"jvb": func() string { return runningStats(5e9, 1000) },
		"stopped": func() string {
			return `{"read":"0001-01-01T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":0}},"memory_stats":{}}`
		},
	}))
	c := newDockerClient("unix://"+fake.socket, time.Second)
	ctx := context.Background()

	cpu, mem, reason := c.stats(ctx, "jvb")
	if reason != "" || cpu != 5e9 || mem != 1000 {
		t.Errorf("jvb: %d %d %q", cpu, mem, reason)
	}
	if _, _, reason := c.stats(ctx, "missing"); reason != scenario.ReasonNotFound {
		t.Errorf("missing: %q, want not_found", reason)
	}
	if _, _, reason := c.stats(ctx, "stopped"); reason != scenario.ReasonNotRunning {
		t.Errorf("stopped: %q, want not_running", reason)
	}
}

func TestDockerClient_Unreachable(t *testing.T) {
	c := newDockerClient("unix:///nonexistent/docker.sock", time.Second)
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != scenario.ReasonDockerUnreachable {
		t.Errorf("no socket: %q, want docker_unreachable", reason)
	}
}

func TestDockerClient_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores socket permissions")
	}
	fake := newFakeDocker(t, containersHandler(nil))
	if err := os.Chmod(fake.socket, 0); err != nil {
		t.Fatal(err)
	}
	c := newDockerClient("unix://"+fake.socket, time.Second)
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != scenario.ReasonPermissionDenied {
		t.Errorf("chmod 000: %q, want permission_denied", reason)
	}
}

func TestDockerClient_Slow(t *testing.T) {
	release := make(chan struct{})
	fake := newFakeDocker(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)
	c := newDockerClient("unix://"+fake.socket, 100*time.Millisecond)
	start := time.Now()
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != scenario.ReasonDockerUnreachable {
		t.Errorf("slow answer: %q, want docker_unreachable", reason)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("took %v, want the 100 ms timeout", d)
	}
}

func TestDockerClient_Host(t *testing.T) {
	if c := newDockerClient("", time.Second); c.socket != defaultDockerSocket || c.remote {
		t.Errorf("default: %+v", c)
	}
	if c := newDockerClient("unix:///run/user/1000/docker.sock", time.Second); c.socket != "/run/user/1000/docker.sock" {
		t.Errorf("unix://: socket %q", c.socket)
	}
	c := newDockerClient("tcp://10.0.0.1:2375", time.Second)
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != scenario.ReasonRemoteDockerHost {
		t.Errorf("tcp://: %q, want remote_docker_host", reason)
	}
}
