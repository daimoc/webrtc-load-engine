package remote

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestJanusRemote_NotBoundToLoopback reads the real platform files: the local
// config binds the bots' WebSocket to 127.0.0.1, which nobody can reach on
// another machine, and a fake runner cannot show that.
func TestJanusRemote_NotBoundToLoopback(t *testing.T) {
	dir := filepath.Join("..", "..", "tests", "platforms", "janus")
	overlay, err := os.ReadFile(filepath.Join(dir, "docker-compose.remote.yml"))
	if err != nil {
		t.Fatal(err)
	}
	const target = "/opt/janus/etc/janus/janus.transport.websockets.jcfg"
	m := regexp.MustCompile(`(?m)^\s*- (\S+):` + regexp.QuoteMeta(target) + `:ro$`).FindSubmatch(overlay)
	if m == nil {
		t.Fatalf("the overlay does not replace %s", target)
	}
	conf, err := os.ReadFile(filepath.Join(dir, string(m[1])))
	if err != nil {
		t.Fatal(err)
	}
	// Without ws_ip Janus binds every interface. It refuses "0.0.0.0" at start:
	// "No interface associated with 0.0.0.0?".
	if regexp.MustCompile(`(?m)^\s*ws_ip\b`).Match(conf) {
		t.Errorf("remote websockets config sets ws_ip:\n%s", conf)
	}
	if !strings.Contains(string(conf), "ws_port = 8188") {
		t.Errorf("remote websockets config changed the port the server table names:\n%s", conf)
	}
}

// TestJitsiRemote_IgnoresBandwidthEstimate: as deployed, the bridge drops
// streams for bots whose bandwidth estimate is low (2.9 Mbps for 4.6 needed
// on a 1 Gb/s LAN, validation-report.md SC-002), which the other servers do
// not do. The user chose on 2026-11-07 to turn that off for remote runs.
func TestJitsiRemote_IgnoresBandwidthEstimate(t *testing.T) {
	overlay, err := os.ReadFile(filepath.Join("..", "..", "tests", "platforms", "docker-jitsi-meet", "docker-compose.remote.yml"))
	if err != nil {
		t.Fatal(err)
	}
	jvb := regexp.MustCompile(`(?ms)^  jvb:\n(.*?)(?:^  \S|\z)`).FindSubmatch(overlay)
	if jvb == nil {
		t.Fatal("no jvb service in the overlay")
	}
	if !regexp.MustCompile(`(?m)^\s+JVB_CC_TRUST_BWE: "false"$`).Match(jvb[1]) {
		t.Errorf("jvb does not set JVB_CC_TRUST_BWE to false:\n%s", jvb[1])
	}
}
