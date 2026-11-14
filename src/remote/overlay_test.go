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

// TestJitsiRemote_DefaultBandwidthEstimate (015 FR-011): the remote bridge
// uses its bandwidth estimate as by default. 013 turned it off
// (JVB_CC_TRUST_BWE=false) because the bridge suspended some bots' streams
// on a LAN; 015 found the bots caused it and corrected them, and the user
// chose on 2026-11-14 to return to the default.
func TestJitsiRemote_DefaultBandwidthEstimate(t *testing.T) {
	overlay, err := os.ReadFile(filepath.Join("..", "..", "tests", "platforms", "docker-jitsi-meet", "docker-compose.remote.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(overlay), "JVB_CC_TRUST_BWE") {
		t.Errorf("the remote overlay sets JVB_CC_TRUST_BWE:\n%s", overlay)
	}
}

// TestOverlays_DeclareTheirSettings (015 FR-012): what a remote overlay sets
// in a service's environment changes the server from its defaults, and the
// reports name it from Server.Settings; an overlay setting not declared, or
// a declared one the overlay no longer sets, would make the reports wrong.
func TestOverlays_DeclareTheirSettings(t *testing.T) {
	envLine := regexp.MustCompile(`^\s+([A-Z][A-Z0-9_]*):\s*"?([^"#]*?)"?\s*$`)
	for _, s := range servers {
		overlay, err := os.ReadFile(filepath.Join("..", "..", "tests", "platforms", s.Folder, "docker-compose.remote.yml"))
		if err != nil {
			t.Fatal(err)
		}
		set := map[string]string{}
		in := false
		indent := 0
		for _, line := range strings.Split(string(overlay), "\n") {
			trimmed := strings.TrimLeft(line, " ")
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				continue
			}
			lead := len(line) - len(trimmed)
			if trimmed == "environment:" {
				in, indent = true, lead
				continue
			}
			if in && lead <= indent {
				in = false
			}
			if m := envLine.FindStringSubmatch(line); in && m != nil {
				set[m[1]] = m[2]
			}
		}
		if len(set) != len(s.Settings) {
			t.Errorf("%s: overlay sets %v, Server.Settings declares %v", s.Name, set, s.Settings)
			continue
		}
		for k, v := range s.Settings {
			if set[k] != v {
				t.Errorf("%s: overlay sets %s=%q, declared %q", s.Name, k, set[k], v)
			}
		}
	}
}
