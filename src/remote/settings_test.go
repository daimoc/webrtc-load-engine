package remote

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsPath(t *testing.T) {
	target, _ := ParseTarget("fetis@192.168.1.95:2222")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got, want := SettingsPath(Options{Target: target}), "/xdg/webrtc-load-engine/sfu-remote/192.168.1.95.env"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/me")
	if got, want := SettingsPath(Options{Target: target}), "/home/me/.config/webrtc-load-engine/sfu-remote/192.168.1.95.env"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
	if got := SettingsPath(Options{Target: target, SettingsDir: "/d"}); got != "/d/192.168.1.95.env" {
		t.Errorf("%q", got)
	}
}

func TestSettingsVars(t *testing.T) {
	target, _ := ParseTarget("fetis@192.168.1.95")
	env := map[string]string{"LIVEKIT_API_KEY": "devkey", "LIVEKIT_API_SECRET": "s3"}
	for name, want := range map[string]map[string]string{
		"jitsi": {
			"JITSI_ROOM_URL": "http://192.168.1.95:8000/loadtest", "JITSI_XMPP_DOMAIN": "meet.jitsi", "JITSI_MUC_DOMAIN": "muc.meet.jitsi",
			"JITSI_WS_URL": "ws://192.168.1.95:8000/xmpp-websocket", "JITSI_SERVER_CONTAINERS": "sfu-jitsi-jvb-1,sfu-jitsi-jicofo-1",
		},
		"livekit":   {"LIVEKIT_URL": "ws://192.168.1.95:7880", "LIVEKIT_API_KEY": "devkey", "LIVEKIT_API_SECRET": "s3", "LIVEKIT_SERVER_CONTAINERS": "livekit-server"},
		"mediasoup": {"MEDIASOUP_URL": "ws://192.168.1.95:4443", "MEDIASOUP_ORIGIN": "http://192.168.1.95:4443", "MEDIASOUP_SERVER_CONTAINERS": "mediasoup-demo"},
		"janus":     {"JANUS_URL": "ws://192.168.1.95:8188", "JANUS_SERVER_CONTAINERS": "janus"},
	} {
		got := settingsVars(lookup(name), target, "192.168.1.95", env)
		want["DOCKER_HOST"] = "ssh://fetis@192.168.1.95"
		if len(got) != len(want) {
			t.Errorf("%s: %v", name, got)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s %s = %q, want %q", name, k, got[k], v)
			}
		}
	}
}

func TestServerURL_IPv6(t *testing.T) {
	if got := serverURL(lookup("janus"), "2001:db8::1"); got != "ws://[2001:db8::1]:8188" {
		t.Errorf("%q", got)
	}
}

func TestWriteSettings_Mode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "h.env")
	if err := writeSettings(path, map[string]string{"A": "1"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %o", info.Mode().Perm())
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSettings(path, map[string]string{"A": "2"}); err != nil {
		t.Fatal(err)
	}
	info, _ = os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("existing file keeps mode %o", info.Mode().Perm())
	}
}

func TestWriteSettings_SourcedByShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.env")
	value := `a b$c'd"e`
	if err := writeSettings(path, map[string]string{"ODD": value, "PLAIN": "x"}); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", `set -a; . "$1"; set +a; printf %s "$ODD"; sh -c 'printf "|%s" "$PLAIN"'`, "sh", path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got, want := string(out), value+"|x"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}

func TestDeploy_WritesSettingsOnSuccessOnly(t *testing.T) {
	f, o := machine(t, "livekit")
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	f.Answer("sfu-remote/current", "", nil)
	f.Answer("/dev/tcp/", "", os.ErrDeadlineExceeded)
	if _, err := Deploy(context.Background(), f, lookup("livekit"), o); err == nil {
		t.Fatal("no error")
	}
	if _, err := os.Stat(SettingsPath(o)); err == nil {
		t.Error("settings written after a failure")
	}
	f.Answer("/dev/tcp/", "", nil)
	if _, err := Deploy(context.Background(), f, lookup("livekit"), o); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(SettingsPath(o))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "LIVEKIT_API_SECRET='") {
		t.Errorf("settings:\n%s", b)
	}
}

func TestRemove_DeletesSettings(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	if err := writeSettings(SettingsPath(o), map[string]string{"A": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := Remove(context.Background(), f, o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SettingsPath(o)); !os.IsNotExist(err) {
		t.Errorf("settings remain: %v", err)
	}
}

func TestStatus_SettingsLines(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	o.Dial = func(string) error { return nil }
	r, err := Status(context.Background(), f, o)
	if err != nil {
		t.Fatal(err)
	}
	out := r.String()
	for _, want := range []string{"url:       ws://192.168.1.95:8188\n", "settings:  " + SettingsPath(o) + "\n", "tls:       not needed (ws://)\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
	for _, a := range []string{"url:", "settings:", "tls:", "reachable:"} {
		if strings.Index(out, "address:") > strings.Index(out, a) {
			t.Errorf("%s before address:\n%s", a, out)
		}
	}
}

func TestTLSNote(t *testing.T) {
	for _, tc := range []struct{ server, url, want string }{
		{"janus", "ws://h:8188", "not needed (ws://)"},
		{"janus", "wss://h:8188", "bots need JANUS_INSECURE_TLS=1 with a self-signed certificate"},
		{"mediasoup", "wss://h:4443", "bots need MEDIASOUP_INSECURE_TLS=1 with a self-signed certificate"},
	} {
		if got := tlsNote(lookup(tc.server), tc.url); got != tc.want {
			t.Errorf("%s %s: %q", tc.server, tc.url, got)
		}
	}
}
