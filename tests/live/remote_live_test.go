//go:build live

package live

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"webrtc-load-engine/src/remote"
)

// TestRemote_DeployStatusRemove drives a real machine (013):
//
//	SFU_REMOTE_HOST=user@host SFU_REMOTE_SERVER=janus \
//	  go test -tags live -count=1 -v -run Remote_ ./tests/live/
//
// SFU_REMOTE_ADDRESS sets the announced address. SFU_REMOTE_EXTERNAL_FIREWALL
// states that a firewall restricts a machine with a public address: only set
// it when one does. The machine ends with nothing deployed.
func TestRemote_DeployStatusRemove(t *testing.T) {
	host := os.Getenv("SFU_REMOTE_HOST")
	if host == "" {
		t.Skip("SFU_REMOTE_HOST not set")
	}
	s, ok := remote.LookupServer(envOr("SFU_REMOTE_SERVER", "janus"))
	if !ok {
		t.Fatalf("unknown server %q", os.Getenv("SFU_REMOTE_SERVER"))
	}
	target, err := remote.ParseTarget(host)
	if err != nil {
		t.Fatal(err)
	}
	o := remote.Options{
		Target:           target,
		Address:          os.Getenv("SFU_REMOTE_ADDRESS"),
		ExternalFirewall: os.Getenv("SFU_REMOTE_EXTERNAL_FIREWALL") != "",
		PlatformsDir:     "../platforms",
		SettingsDir:      t.TempDir(),
		Log:              os.Stderr,
	}
	r := remote.SSHRunner{Target: target}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	t.Cleanup(func() {
		if err := remote.Remove(context.Background(), r, o); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	report, err := remote.Deploy(ctx, r, s, o)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != "running" {
		t.Fatalf("state %q:\n%s", report.State, report)
	}
	info, err := os.Stat(remote.SettingsPath(o))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("settings mode %o", info.Mode().Perm())
	}
	settings, err := os.ReadFile(remote.SettingsPath(o))
	if err != nil {
		t.Fatal(err)
	}
	ps, err := r.Run(ctx, "ps -eo args", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(settings), "\n") {
		key, value, _ := strings.Cut(line, "=")
		value = strings.Trim(value, "'")
		if !strings.Contains(key, "SECRET") || value == "" {
			continue
		}
		if strings.Contains(report.String(), value) {
			t.Errorf("status prints %s", key)
		}
		if bytes.Contains(ps, []byte(value)) {
			t.Errorf("%s shows in the machine's ps", key)
		}
	}

	if err := remote.Remove(ctx, r, o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(remote.SettingsPath(o)); !os.IsNotExist(err) {
		t.Errorf("settings file remains: %v", err)
	}
	out, err := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", host, "docker ps -a --filter label=com.docker.compose.project="+"sfu-"+s.Name+" --format '{{.Names}}'").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("containers remain: %s", out)
	}
}
