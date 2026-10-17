package hostload

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"webrtc-load-engine/src/scenario"
)

// fakeClock is advanced by the test.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) advance(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// counterBody returns a stats body whose CPU counter is *cpu seconds.
func counterBody(cpu *atomic.Int64, mem int64) func() string {
	return func() string { return runningStats(uint64(cpu.Load())*1e9, mem) }
}

type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *logBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

func newTestSampler(t *testing.T, fake *fakeDocker, containers []string, logs *logBuffer) (*Sampler, *fakeClock) {
	t.Helper()
	var cfg *scenario.ServerMonitoringConfig
	if containers != nil {
		cfg = &scenario.ServerMonitoringConfig{Containers: containers, IntervalSeconds: 5}
	}
	var w interface{ Write([]byte) (int, error) } = &logBuffer{}
	if logs != nil {
		w = logs
	}
	s := NewSampler(uuid.NewString(), cfg, slog.New(slog.NewTextHandler(w, nil)))
	clock := &fakeClock{t: t0}
	s.now = clock.now
	s.registry = newOverlapRegistry()
	if fake != nil {
		s.docker = newDockerClient("unix://"+fake.socket, time.Second)
	}
	return s, clock
}

func TestSampler_Containers(t *testing.T) {
	var jvbCPU, jicofoCPU atomic.Int64
	fake := newFakeDocker(t, containersHandler(map[string]func() string{
		"jvb":    counterBody(&jvbCPU, 2000),
		"jicofo": counterBody(&jicofoCPU, 500),
	}))
	logs := &logBuffer{}
	s, clock := newTestSampler(t, fake, []string{"jvb", "missing", "jicofo"}, logs)
	ctx := context.Background()
	s.start()

	s.tick(ctx) // Baseline at t0
	for i := 0; i < 3; i++ {
		clock.advance(5 * time.Second)
		jvbCPU.Add(10)   // 2 threads over 5 s
		jicofoCPU.Add(5) // 1 thread
		s.tick(ctx)
	}
	if v := testutil.ToFloat64(ServerCPUThreads.WithLabelValues(s.scenarioID, "jvb")); v != 2 {
		t.Errorf("jvb gauge %v, want 2", v)
	}
	if v := testutil.ToFloat64(ServerMemoryBytes.WithLabelValues(s.scenarioID, "jicofo")); v != 500 {
		t.Errorf("jicofo memory gauge %v, want 500", v)
	}
	s.finish()
	server := s.Summary()

	if !server.Available || server.IntervalSeconds != 5 || server.Reason != "" {
		t.Errorf("server %+v", server)
	}
	if server.Period == nil || !server.Period.Start.Equal(t0) || !server.Period.End.Equal(t0.Add(15*time.Second)) {
		t.Errorf("period %+v", server.Period)
	}
	if len(server.Containers) != 3 || server.Containers[0].Name != "jvb" || server.Containers[1].Name != "missing" || server.Containers[2].Name != "jicofo" {
		t.Fatalf("containers not in test plan order: %+v", server.Containers)
	}
	jvb, missing, jicofo := server.Containers[0], server.Containers[1], server.Containers[2]
	if jvb.Status != scenario.LoadStatusMeasured || jvb.CPUThreads.Average != 2 || jvb.CPUThreads.Peak != 2 || jvb.Samples != 4 || jvb.MemoryBytes.Peak != 2000 {
		t.Errorf("jvb %+v", jvb)
	}
	if missing.Status != scenario.LoadStatusUnavailable || missing.Reason != scenario.ReasonNotFound || missing.Samples != 0 {
		t.Errorf("missing %+v", missing)
	}
	if jicofo.Status != scenario.LoadStatusMeasured || jicofo.CPUThreads.Average != 1 {
		t.Errorf("jicofo %+v", jicofo)
	}
	total := server.Total
	if total == nil || total.ContainersMeasured != 2 || total.CPUThreads.Average != 3 || total.CPUThreads.Peak != 3 || total.MemoryBytes.Peak != 2500 {
		t.Errorf("total %+v", total)
	}
	if n := strings.Count(logs.String(), "container=missing"); n != 1 {
		t.Errorf("missing container logged %d times, want once:\n%s", n, logs)
	}
	// Series deleted at the end of the run.
	if n := testutil.CollectAndCount(ServerCPUThreads); n != 0 && testutil.ToFloat64(ServerCPUThreads.WithLabelValues(s.scenarioID, "jvb")) != 0 {
		t.Errorf("series kept after the run")
	}
}

func TestSampler_PeakOfSum(t *testing.T) {
	var a, b atomic.Int64
	fake := newFakeDocker(t, containersHandler(map[string]func() string{
		"a": counterBody(&a, 1), "b": counterBody(&b, 1),
	}))
	s, clock := newTestSampler(t, fake, []string{"a", "b"}, nil)
	ctx := context.Background()
	s.start()
	s.tick(ctx)
	clock.advance(time.Second)
	a.Add(4) // a peaks first
	s.tick(ctx)
	clock.advance(time.Second)
	b.Add(4) // then b
	s.tick(ctx)
	s.finish()
	server := s.Summary()
	// Sum of peaks would be 8; the busiest interval had 4.
	if p := server.Total.CPUThreads.Peak; p != 4 {
		t.Errorf("total peak %v, want 4 (peak of the sum)", p)
	}
}

func TestSampler_PartialAndShared(t *testing.T) {
	var cpu atomic.Int64
	running := atomic.Bool{}
	running.Store(true)
	fake := newFakeDocker(t, containersHandler(map[string]func() string{
		"jvb": func() string {
			if !running.Load() {
				return `{"read":"0001-01-01T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":0}},"memory_stats":{}}`
			}
			return runningStats(uint64(cpu.Load())*1e9, 1)
		},
	}))
	s, clock := newTestSampler(t, fake, []string{"jvb"}, nil)
	stopOther := s.registry.watch("other", []string{"jvb"})
	defer stopOther()
	ctx := context.Background()
	s.start()
	s.tick(ctx)
	running.Store(false)
	clock.advance(5 * time.Second)
	s.tick(ctx)
	running.Store(true)
	clock.advance(5 * time.Second)
	s.tick(ctx)
	s.finish()
	server := s.Summary()
	jvb := server.Containers[0]
	if jvb.Status != scenario.LoadStatusPartial || jvb.Reason != scenario.ReasonNotRunning || jvb.Missed != 1 || jvb.Samples != 2 {
		t.Errorf("jvb %+v", jvb)
	}
	if len(jvb.SharedWith) != 1 || jvb.SharedWith[0] != "other" {
		t.Errorf("shared_with %v, want [other]", jvb.SharedWith)
	}
}

func TestSampler_AllUnavailable(t *testing.T) {
	s, _ := newTestSampler(t, nil, []string{"jvb", "jicofo"}, nil)
	s.docker = newDockerClient("unix:///nonexistent.sock", time.Second)
	s.start()
	s.tick(context.Background())
	s.finish()
	server := s.Summary()
	if server.Available || server.Reason != scenario.ReasonDockerUnreachable || server.Total != nil {
		t.Errorf("server %+v, want unavailable, docker_unreachable", server)
	}
}

func TestSampler_NotRequested(t *testing.T) {
	requests := atomic.Int32{}
	fake := newFakeDocker(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1) })
	s, _ := newTestSampler(t, fake, nil, nil)
	s.start()
	s.tick(context.Background())
	s.finish()
	server := s.Summary()
	if server.Available || server.Reason != scenario.ReasonNotRequested || len(server.Containers) != 0 {
		t.Errorf("server %+v, want not_requested", server)
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("%d Docker requests without containers", n)
	}
}

func TestSampler_SkippedTick(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	fake := newFakeDocker(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 2 {
			<-release // Second tick hangs
		}
		w.Write([]byte(runningStats(1, 1)))
	})
	s, _ := newTestSampler(t, fake, []string{"jvb"}, nil)
	ctx := context.Background()
	s.start()
	s.tick(ctx)
	done := make(chan struct{})
	go func() { s.tick(ctx); close(done) }()
	for calls.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	s.tick(ctx) // Still running: skipped
	close(release)
	<-done
	s.finish()
	server := s.Summary()
	if jvb := server.Containers[0]; jvb.Missed != 1 || jvb.Samples != 2 {
		t.Errorf("jvb %+v, want 2 samples and 1 skipped tick", jvb)
	}
}

func TestSampler_Run(t *testing.T) {
	var cpu atomic.Int64
	fake := newFakeDocker(t, containersHandler(map[string]func() string{"jvb": counterBody(&cpu, 1)}))
	s, _ := newTestSampler(t, fake, []string{"jvb"}, nil)
	s.now = time.Now
	s.interval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	time.Sleep(110 * time.Millisecond)
	cancel()
	<-done
	server := s.Summary()
	// Baseline, about 5 ticks, and the end sample.
	if n := server.Containers[0].Samples; n < 4 || n > 8 {
		t.Errorf("%d samples in 110 ms at 20 ms", n)
	}
}

func TestSampler_Engine(t *testing.T) {
	var cpu atomic.Int64
	fake := newFakeDocker(t, containersHandler(map[string]func() string{"jvb": counterBody(&cpu, 1)}))
	s, clock := newTestSampler(t, fake, []string{"jvb"}, nil)
	var procCPU atomic.Int64
	s.process = func() (uint64, int64, error) { return uint64(procCPU.Load()) * 1e9, 4096, nil }
	ctx := context.Background()
	s.start()
	s.tick(ctx)
	clock.advance(5 * time.Second)
	procCPU.Add(2) // 0.4 thread
	s.tick(ctx)
	if v := testutil.ToFloat64(EngineCPUThreads.WithLabelValues(s.scenarioID)); v != 0.4 {
		t.Errorf("engine gauge %v, want 0.4", v)
	}
	s.finish()
	server := s.Summary()
	m := s.LoadMachine()
	if m.Period == nil || *m.Period != *server.Period {
		t.Errorf("load machine period %v, server period %v", m.Period, server.Period)
	}
	if m.Engine.CPUThreads.Average != 0.4 || m.Engine.MemoryBytes.Peak != 4096 || m.Engine.Samples != 2 {
		t.Errorf("engine %+v", m.Engine)
	}
	if m.HardwareThreads < 1 || m.UsableThreads < 1 {
		t.Errorf("threads %d/%d", m.HardwareThreads, m.UsableThreads)
	}
}

func TestSampler_EngineWithoutContainers(t *testing.T) {
	s, clock := newTestSampler(t, nil, nil, nil)
	if s.interval != 5*time.Second {
		t.Errorf("default interval %v", s.interval)
	}
	s.start()
	s.tick(context.Background())
	clock.advance(5 * time.Second)
	s.tick(context.Background())
	s.finish()
	if m := s.LoadMachine(); m.Engine.Samples != 2 || m.Period == nil {
		t.Errorf("engine without containers: %+v", m)
	}
}

// BenchmarkTick_Docker measures the CPU one tick costs the engine against
// real containers (SC-004): HOSTLOAD_DOCKER_CONTAINERS="jvb,jicofo,prosody"
// go test -run '^$' -bench Tick_Docker ./src/hostload/
func BenchmarkTick_Docker(b *testing.B) {
	names := os.Getenv("HOSTLOAD_DOCKER_CONTAINERS")
	if names == "" {
		b.Skip("HOSTLOAD_DOCKER_CONTAINERS not set")
	}
	s := NewSampler(uuid.NewString(), &scenario.ServerMonitoringConfig{Containers: strings.Split(names, ","), IntervalSeconds: 5}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.start()
	defer s.finish()
	ctx := context.Background()
	s.tick(ctx) // Opens the connections
	before, _, _ := processSample()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.tick(ctx)
	}
	b.StopTimer()
	after, _, _ := processSample()
	cpuPerTick := float64(after-before) / float64(b.N)
	b.ReportMetric(cpuPerTick/1e6, "cpu-ms/tick")
	// At the default interval, one tick every 5 s.
	b.ReportMetric(cpuPerTick/5e9, "threads@5s")
	if server := s.Summary(); server.Total == nil || server.Total.ContainersMeasured != len(s.containers) {
		b.Fatalf("not every container measured: %+v", server.Containers)
	}
}
