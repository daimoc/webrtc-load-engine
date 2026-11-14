//go:build live

package browser

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestLive_Page runs the real browser on BROWSER_LIVE_URL for
// BROWSER_LIVE_SECONDS (default 20) and logs its result:
//
//	BROWSER_LIVE_URL='https://localhost:8443/room#config.prejoinConfig.enabled=false' BROWSER_LIVE_INSECURE_TLS=1 \
//	go test -tags live -count=1 -v -run Live_Page ./src/browser/
func TestLive_Page(t *testing.T) {
	u := os.Getenv("BROWSER_LIVE_URL")
	if u == "" {
		t.Skip("BROWSER_LIVE_URL not set")
	}
	d := 20 * time.Second
	if v := os.Getenv("BROWSER_LIVE_SECONDS"); v != "" {
		s, err := time.ParseDuration(v + "s")
		if err != nil {
			t.Fatal(err)
		}
		d = s
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := Start(context.Background(), Options{URL: u, Window: [2]int{1280, 720}, InsecureTLS: os.Getenv("BROWSER_LIVE_INSECURE_TLS") == "1"}, logger)
	time.Sleep(d)
	r := s.Stop()
	t.Logf("status %s %s, streams %d, heights %v, samples %d, kbps %+v, freezes %d",
		r.Status, r.Reason, r.VideoStreams, r.HeightShare, len(r.Samples), r.ReceivedBitrateKbps, r.FreezesTotal)
	if r.CPUThreadsAverage != nil {
		t.Logf("cpu %.3f threads", *r.CPUThreadsAverage)
	}
	for i, smp := range r.Samples {
		if i < 5 || i == len(r.Samples)-1 {
			t.Logf("sample %+v", smp)
		}
	}
}
