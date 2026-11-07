package hostload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

// fakeSSH puts a fake ssh first in PATH. It records its arguments and its
// process ID, then answers each HTTP request on stdio with stats, as
// docker system dial-stdio would, until stdin closes.
func fakeSSH(t *testing.T, script string) (dir string) {
	t.Helper()
	dir = t.TempDir()
	body := fmt.Sprintf("#!/bin/bash\nprintf '%%s\\n' \"$@\" > %[1]s/args\necho $$ >> %[1]s/pids\n%[2]s\n", dir, script)
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// serveStats answers every request with a stats body, keeping the connection.
func serveStats(cpu uint64, mem int64) string {
	b := runningStats(cpu, mem)
	return fmt.Sprintf(`while IFS= read -r line; do
  line=${line%%$'\r'}
  if [ -z "$line" ]; then printf 'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%%s' '%s'; fi
done`, len(b), b)
}

func pids(t *testing.T, dir string) []int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "pids"))
	if err != nil {
		return nil
	}
	var out []int
	for _, f := range strings.Fields(string(b)) {
		var p int
		fmt.Sscan(f, &p)
		out = append(out, p)
	}
	return out
}

// gone says whether a process has ended and been reaped.
func gone(pid int) bool { return syscall.Kill(pid, 0) == syscall.ESRCH }

func waitGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !gone(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("process %d still there", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func sshClient(t *testing.T, host string, timeout time.Duration) *dockerClient {
	t.Helper()
	c := newDockerClient(host, timeout)
	t.Cleanup(c.close)
	return c
}

func TestSSH_Stats(t *testing.T) {
	fakeSSH(t, serveStats(1500, 4096))
	c := sshClient(t, "ssh://me@box:2222", time.Second)
	cpu, mem, reason := c.stats(context.Background(), "jvb")
	if reason != "" || cpu != 1500 || mem != 4096 {
		t.Errorf("cpu %d mem %d reason %q", cpu, mem, reason)
	}
}

func TestSSH_Arguments(t *testing.T) {
	dir := fakeSSH(t, serveStats(1, 1))
	c := sshClient(t, "ssh://me@box:2222", time.Second)
	c.stats(context.Background(), "jvb")
	b, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(strings.Fields(string(b)), " ")
	want := "-o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=15 -p 2222 -- me@box docker system dial-stdio"
	if got != want {
		t.Errorf("%q, want %q", got, want)
	}
}

func TestSSH_NoPortNoUser(t *testing.T) {
	dir := fakeSSH(t, serveStats(1, 1))
	c := sshClient(t, "ssh://box", time.Second)
	c.stats(context.Background(), "jvb")
	b, _ := os.ReadFile(filepath.Join(dir, "args"))
	got := strings.Join(strings.Fields(string(b)), " ")
	if !strings.HasSuffix(got, "ServerAliveInterval=15 -- box docker system dial-stdio") {
		t.Errorf("%q", got)
	}
}

func TestSSH_ReusesOneProcess(t *testing.T) {
	dir := fakeSSH(t, serveStats(1, 1))
	c := sshClient(t, "ssh://box", time.Second)
	for range 3 {
		if _, _, reason := c.stats(context.Background(), "jvb"); reason != "" {
			t.Fatal(reason)
		}
	}
	if got := len(pids(t, dir)); got != 1 {
		t.Errorf("%d ssh processes", got)
	}
}

func TestSSH_ConcurrentContainersStayConnected(t *testing.T) {
	dir := fakeSSH(t, "sleep 0.05\n"+serveStats(1, 1))
	c := sshClient(t, "ssh://box", 2*time.Second)
	sample := func() {
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, _, reason := c.stats(context.Background(), "jvb"); reason != "" {
					t.Error(reason)
				}
			}()
		}
		wg.Wait()
	}
	sample()
	first := len(pids(t, dir))
	sample()
	if got := len(pids(t, dir)); got != first || first > 8 {
		t.Errorf("first %d, then %d processes", first, got)
	}
}

// The dialling request's context ending must not end the connection.
func TestSSH_ProcessOutlivesTheRequest(t *testing.T) {
	dir := fakeSSH(t, serveStats(1, 1))
	c := sshClient(t, "ssh://box", time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	if _, _, reason := c.stats(ctx, "jvb"); reason != "" {
		t.Fatal(reason)
	}
	cancel()
	time.Sleep(100 * time.Millisecond)
	p := pids(t, dir)
	if len(p) != 1 || gone(p[0]) {
		t.Fatalf("process ended with its request: %v", p)
	}
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != "" {
		t.Fatal(reason)
	}
	if got := len(pids(t, dir)); got != 1 {
		t.Errorf("%d processes", got)
	}
}

func TestSSH_FailureIsUnreachable(t *testing.T) {
	fakeSSH(t, "exit 255")
	c := sshClient(t, "ssh://box", time.Second)
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != scenario.ReasonDockerUnreachable {
		t.Errorf("reason %q", reason)
	}
}

func TestSSH_SlowServer(t *testing.T) {
	fakeSSH(t, "exec sleep 30")
	c := sshClient(t, "ssh://box", time.Second)
	c.timeout = 100 * time.Millisecond
	start := time.Now()
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != scenario.ReasonDockerUnreachable {
		t.Errorf("reason %q", reason)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

func TestSSH_Timeout(t *testing.T) {
	if c := newDockerClient("ssh://box", 2*time.Second); c.timeout != sshRequestTimeout {
		t.Errorf("ssh timeout %v", c.timeout)
	}
	if c := newDockerClient("", time.Second); c.timeout != time.Second {
		t.Errorf("local timeout %v", c.timeout)
	}
	if sshRequestTimeout != 4*time.Second {
		t.Errorf("%v", sshRequestTimeout)
	}
}

func TestSSH_CloseEndsTheProcess(t *testing.T) {
	dir := fakeSSH(t, serveStats(1, 1))
	c := newDockerClient("ssh://box", time.Second)
	if _, _, reason := c.stats(context.Background(), "jvb"); reason != "" {
		t.Fatal(reason)
	}
	p := pids(t, dir)
	if len(p) != 1 {
		t.Fatalf("pids %v", p)
	}
	c.close()
	waitGone(t, p[0]) // gone also means reaped: a zombie still answers kill 0
}

func TestSSH_ClosingLocalClientIsHarmless(t *testing.T) {
	newDockerClient("", time.Second).close()
	newDockerClient("tcp://10.0.0.1:2375", time.Second).close()
}
