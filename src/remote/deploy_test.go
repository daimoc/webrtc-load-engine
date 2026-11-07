package remote

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// machine returns a fake of the validation machine with server s running
// under this project and nothing deployed.
func machine(t *testing.T, name string) (*FakeRunner, Options) {
	t.Helper()
	f := &FakeRunner{}
	f.Answer("uname -m", goodFacts, nil)
	f.Answer("ip -j addr", string(addrJSON("192.168.1.95")), nil)
	s, ok := LookupServer(name)
	if !ok {
		t.Fatal(name)
	}
	f.Answer("docker inspect", inspectOutput(s, "running"), nil)
	target, _ := ParseTarget("fetis@192.168.1.95")
	return f, Options{Target: target, PlatformsDir: platformsDir, Poll: time.Millisecond, Wait: 50 * time.Millisecond, SettingsDir: t.TempDir()}
}

func inspectOutput(s Server, state string) string {
	var b strings.Builder
	for c, image := range s.Images {
		b.WriteString("/" + c + "|" + state + "|" + image + "|" + s.project() + "\n")
	}
	return b.String()
}

func lookup(name string) Server { s, _ := LookupServer(name); return s }

func TestDeploy_Janus(t *testing.T) {
	f, o := machine(t, "janus")
	if _, err := Deploy(context.Background(), f, lookup("janus"), o); err != nil {
		t.Fatal(err)
	}
	up := f.index("up -d --build")
	if up < 0 {
		t.Fatalf("no compose up in %v", f.commands())
	}
	want := "docker compose -p sfu-janus -f docker-compose.yml -f docker-compose.remote.yml up -d --build janus"
	if c := f.commands()[up]; !strings.Contains(c, want) {
		t.Errorf("compose: %s", c)
	}
	for _, c := range f.commands() {
		if strings.Contains(c, "up -d") && strings.Contains(c, "janus-demos") {
			t.Errorf("demos started: %s", c)
		}
	}
	if f.index("/dev/tcp/127.0.0.1/8188") < up {
		t.Error("did not wait for the signaling port after starting")
	}
}

func TestDeploy_FailedPrerequisiteChangesNothing(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("uname -m", factsWith("docker=ok", "docker=no"), nil)
	if _, err := Deploy(context.Background(), f, lookup("janus"), o); err == nil {
		t.Fatal("no error")
	}
	assertNoChange(t, f)
}

func assertNoChange(t *testing.T, f *FakeRunner) {
	t.Helper()
	for _, c := range f.commands() {
		for _, w := range []string{"mkdir", "tar ", "cat >", "compose -p", "iptables -", "rm -"} {
			if strings.Contains(c, w) {
				t.Errorf("changed the machine: %s", c)
			}
		}
	}
}

func TestDeploy_RefusesPublicMachine(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("ip -j addr", string(addrJSON("100.64.0.1", "203.0.113.7")), nil)
	_, err := Deploy(context.Background(), f, lookup("janus"), o)
	want := "203.0.113.7 is a public address, and the servers listen on every interface.\nGive --allow <your-cidr>, or --external-firewall if a firewall already restricts it."
	if err == nil || err.Error() != want {
		t.Errorf("err = %v", err)
	}
	assertNoChange(t, f)
}

func TestDeploy_PublicMachineWithExternalFirewall(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("ip -j addr", string(addrJSON("203.0.113.7")), nil)
	o.ExternalFirewall = true
	o.Address = "203.0.113.7"
	if _, err := Deploy(context.Background(), f, lookup("janus"), o); err != nil {
		t.Fatal(err)
	}
	if f.index("iptables") >= 0 {
		t.Error("rules installed")
	}
	if !strings.Contains(string(stdinOf(t, f, "/access")), "mode=external") {
		t.Error("access not recorded")
	}
}

func TestDeploy_AllowInstallsRulesBeforeStarting(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("uname -m", factsWith("sudo=no", "sudo=ok"), nil)
	f.Answer("ip -j addr", string(addrJSON("203.0.113.7")), nil)
	o.Allow = []netip.Prefix{netip.MustParsePrefix("198.51.100.4/32")}
	o.Address = "203.0.113.7"
	if _, err := Deploy(context.Background(), f, lookup("janus"), o); err != nil {
		t.Fatal(err)
	}
	rules, up := f.index("-A SFU-REMOTE -s 198.51.100.4/32"), f.index("up -d")
	if rules < 0 || rules > up {
		t.Errorf("rules at %d, up at %d", rules, up)
	}
	if !strings.Contains(string(stdinOf(t, f, "/access")), "allowed=198.51.100.4/32") {
		t.Error("allowed not recorded")
	}
}

func TestDeploy_NeedsAnAddress(t *testing.T) {
	f, o := machine(t, "janus")
	o.Target, _ = ParseTarget("my-alias")
	if _, err := Deploy(context.Background(), f, lookup("janus"), o); err == nil || !strings.Contains(err.Error(), "--address") {
		t.Errorf("err = %v", err)
	}
	assertNoChange(t, f)
}

func TestDeploy_RemovesCurrentFirst(t *testing.T) {
	f, o := machine(t, "livekit")
	f.Answer("sfu-remote/current", "janus\n", nil)
	if _, err := Deploy(context.Background(), f, lookup("livekit"), o); err != nil {
		t.Fatal(err)
	}
	down, up := f.index("compose -p sfu-janus down"), f.index("compose -p sfu-livekit")
	if down < 0 || down > up {
		t.Errorf("down at %d, up at %d", down, up)
	}
}

func TestDeploy_MarkerBeforeStart(t *testing.T) {
	f, o := machine(t, "janus")
	boom := errors.New("build failed")
	f.Answer("up -d --build", "", boom)
	if _, err := Deploy(context.Background(), f, lookup("janus"), o); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	marker := slices.IndexFunc(f.calls, func(c fakeCall) bool {
		return strings.Contains(c.cmd, `/current`) && strings.Contains(c.cmd, "cat >") && string(c.stdin) == "janus\n"
	})
	if up := f.index("up -d --build"); marker < 0 || marker > up {
		t.Errorf("marker at %d, up at %d", marker, up)
	}
}

// stdinOf returns what the write to path received.
func stdinOf(t *testing.T, f *FakeRunner, path string) []byte {
	t.Helper()
	for _, c := range f.calls {
		if strings.Contains(c.cmd, path) && strings.Contains(c.cmd, "cat >") {
			return c.stdin
		}
	}
	t.Fatalf("no write to %s in %v", path, f.commands())
	return nil
}

func TestDeploy_SecretsOnlyOnStdin(t *testing.T) {
	f, o := machine(t, "livekit")
	if _, err := Deploy(context.Background(), f, lookup("livekit"), o); err != nil {
		t.Fatal(err)
	}
	env := ParseEnv(stdinOf(t, f, "/.env"))
	secret := env["LIVEKIT_API_SECRET"]
	if len(secret) != 32 {
		t.Fatalf("secret %q", secret)
	}
	for _, c := range f.commands() {
		if strings.Contains(c, secret) {
			t.Errorf("secret on a command line: %s", c)
		}
	}
	for _, c := range f.calls {
		if strings.Contains(c.cmd, "/.env") && strings.Contains(c.cmd, "cat >") {
			if !strings.Contains(c.cmd, "umask 077") || !strings.Contains(c.cmd, "chmod 600") {
				t.Errorf(".env mode: %s", c.cmd)
			}
		}
	}
}

func TestDeploy_ReusesSecrets(t *testing.T) {
	f, o := machine(t, "livekit")
	f.Answer("sfu-remote/current", "livekit\n", nil)
	f.Answer(`cat "$HOME/sfu-remote/livekit/.env"`, "LIVEKIT_API_SECRET=abc\nLIVEKIT_API_KEY=devkey\n", nil)
	if _, err := Deploy(context.Background(), f, lookup("livekit"), o); err != nil {
		t.Fatal(err)
	}
	if got := ParseEnv(stdinOf(t, f, "/.env"))["LIVEKIT_API_SECRET"]; got != "abc" {
		t.Errorf("secret %q", got)
	}
	if f.index("rm -rf ") >= 0 {
		t.Error("same server's folder deleted first")
	}
}

// A redeploy that changes the settings recreates containers; without a down
// first, each recreated container leaves its old anonymous volume behind,
// where remove no longer sees it (validation-report.md, browser join).
func TestDeploy_SameServerDropsItsVolumesFirst(t *testing.T) {
	f, o := machine(t, "jitsi")
	f.Answer("sfu-remote/current", "jitsi\n", nil)
	if _, err := Deploy(context.Background(), f, lookup("jitsi"), o); err != nil {
		t.Fatal(err)
	}
	down, up := f.index("compose -p sfu-jitsi down -v "), f.index(" up -d ")
	if down < 0 || down > up {
		t.Errorf("down -v at %d, up at %d: %v", down, up, f.commands())
	}
}

func TestDeploy_ForeignContainerIsNotReplaced(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("docker inspect", "/janus|running|someone/janus:1|<no value>\n", nil)
	_, err := Deploy(context.Background(), f, lookup("janus"), o)
	if err == nil || !strings.Contains(err.Error(), "janus") {
		t.Fatalf("err = %v", err)
	}
	assertNoChange(t, f)
}

func TestDeploy_WaitNamesTheContainer(t *testing.T) {
	f, o := machine(t, "jitsi")
	s := lookup("jitsi")
	out := strings.Replace(inspectOutput(s, "running"), "/sfu-jitsi-jvb-1|running", "/sfu-jitsi-jvb-1|exited", 1)
	f.Answer("docker inspect", out, nil)
	_, err := Deploy(context.Background(), f, s, o)
	if err == nil || !strings.Contains(err.Error(), "sfu-jitsi-jvb-1") {
		t.Errorf("err = %v", err)
	}
}

// TestDeploy_ExitedContainerStopsTheWait: with restart "no", an exited
// container never comes back, so waiting the full time only hides the failure.
func TestDeploy_ExitedContainerStopsTheWait(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("docker inspect", inspectOutput(lookup("janus"), "exited"), nil)
	o.Wait = time.Hour
	start := time.Now()
	_, err := Deploy(context.Background(), f, lookup("janus"), o)
	if err == nil {
		t.Fatal("deploy succeeded with an exited container")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("deploy waited %v for a container that cannot come back", took)
	}
	for _, want := range []string{"janus", "exited", "docker logs janus"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestDeploy_WaitsForJitsiFocus: Jitsi's ports open before jicofo has a
// bridge, and a bot joining then gets "conference focus unavailable".
func TestDeploy_WaitsForJitsiFocus(t *testing.T) {
	f, o := machine(t, "jitsi")
	f.Answer("8888/debug", "", errors.New("exit status 1"))
	_, err := Deploy(context.Background(), f, lookup("jitsi"), o)
	if err == nil || !strings.Contains(err.Error(), "jicofo has no operational bridge") {
		t.Errorf("err = %v", err)
	}
	if _, err := os.Stat(SettingsPath(o)); err == nil {
		t.Error("settings written for a server bots cannot join")
	}
}

func TestDeploy_WaitNamesTheSignalingPort(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("/dev/tcp/", "", errors.New("refused"))
	_, err := Deploy(context.Background(), f, lookup("janus"), o)
	if err == nil || !strings.Contains(err.Error(), "8188") {
		t.Errorf("err = %v", err)
	}
}

func TestDeploy_ImageMismatchFailsAndRemoves(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("docker inspect", "/janus|running|webrtc-load-engine/janus:v9|sfu-janus\n", nil)
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	_, err := Deploy(context.Background(), f, lookup("janus"), o)
	if err == nil || !strings.Contains(err.Error(), "v9") {
		t.Fatalf("err = %v", err)
	}
	if f.index("compose -p sfu-janus down") < 0 {
		t.Error("not removed")
	}
}

func TestDeploy_JitsiRunsSetupAfterEnv(t *testing.T) {
	f, o := machine(t, "jitsi")
	if _, err := Deploy(context.Background(), f, lookup("jitsi"), o); err != nil {
		t.Fatal(err)
	}
	env := slices.IndexFunc(f.calls, func(c fakeCall) bool {
		return strings.Contains(c.cmd, "/.env") && strings.Contains(c.cmd, "cat >")
	})
	setup, up := f.index("setup.sh"), f.index("up -d")
	if env < 0 || setup < env || up < setup {
		t.Errorf("env %d setup %d up %d", env, setup, up)
	}
}

func tarNames(t *testing.T, b []byte) map[string]tar.Header {
	t.Helper()
	out := map[string]tar.Header{}
	r := tar.NewReader(bytes.NewReader(b))
	for {
		h, err := r.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out[h.Name] = *h
	}
}

func TestBundle_LeavesSecretsBehind(t *testing.T) {
	dir := t.TempDir()
	for name, mode := range map[string]os.FileMode{
		"docker-compose.yml": 0o644, "docker-compose.remote.yml": 0o644, "env.example": 0o644, "setup.sh": 0o755,
		".env": 0o600, "certs/.gitkeep": 0o644, "certs/key.pem": 0o600, "certs/cert.pem": 0o644, "certs/x.key": 0o600,
		"conf/janus.jcfg": 0o644,
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), mode); err != nil {
			t.Fatal(err)
		}
	}
	b, err := Bundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := tarNames(t, b)
	for _, gone := range []string{".env", "certs/key.pem", "certs/cert.pem", "certs/x.key"} {
		if _, ok := got[gone]; ok {
			t.Errorf("%s was sent", gone)
		}
	}
	for _, want := range []string{"docker-compose.remote.yml", "docker-compose.yml", "certs/.gitkeep", "conf/janus.jcfg"} {
		if _, ok := got[want]; !ok {
			t.Errorf("%s missing", want)
		}
	}
	if h := got["setup.sh"]; h.Mode&0o100 == 0 {
		t.Errorf("setup.sh mode %o", h.Mode)
	}
}

func TestBundle_NeedsOverlay(t *testing.T) {
	if _, err := Bundle(t.TempDir()); err == nil {
		t.Error("no error")
	}
}

func TestDeploy_SendsTheBundle(t *testing.T) {
	f, o := machine(t, "mediasoup")
	if _, err := Deploy(context.Background(), f, lookup("mediasoup"), o); err != nil {
		t.Fatal(err)
	}
	i := f.index("tar -xf -")
	if i < 0 {
		t.Fatal("no tar")
	}
	got := tarNames(t, f.calls[i].stdin)
	if _, ok := got["docker-compose.remote.yml"]; !ok {
		t.Errorf("overlay not sent: %v", got)
	}
	if _, ok := got["certs/key.pem"]; ok {
		t.Error("private key sent")
	}
}

func TestStatus(t *testing.T) {
	access := "mode=private\naddress=192.168.1.95\nallowed=\n"
	for _, tc := range []struct {
		name, current, inspect, want string
	}{
		{"none", "", "", "none"},
		{"running", "janus\n", "/janus|running|webrtc-load-engine/janus:v1.4.3|sfu-janus\n", "running"},
		{"stopped", "janus\n", "/janus|exited|webrtc-load-engine/janus:v1.4.3|sfu-janus\n", "stopped"},
		{"failed, no container", "janus\n", "", "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, o := machine(t, "janus")
			f.Answer("sfu-remote/current", tc.current, nil)
			f.Answer("docker inspect", tc.inspect, nil)
			f.Answer("/access", access, nil)
			o.Dial = func(string) error { return nil }
			r, err := Status(context.Background(), f, o)
			if err != nil {
				t.Fatal(err)
			}
			if r.State != tc.want {
				t.Errorf("state %q, want %q", r.State, tc.want)
			}
		})
	}
}

func TestStatus_PartlyRunningIsFailed(t *testing.T) {
	f, o := machine(t, "jitsi")
	s := lookup("jitsi")
	f.Answer("sfu-remote/current", "jitsi\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	f.Answer("docker inspect", strings.Replace(inspectOutput(s, "running"), "jvb-1|running", "jvb-1|exited", 1), nil)
	o.Dial = func(string) error { return nil }
	r, err := Status(context.Background(), f, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.State != "failed" {
		t.Errorf("state %q", r.State)
	}
}

func TestStatus_NamesUnreachablePorts(t *testing.T) {
	f, o := machine(t, "livekit")
	f.Answer("sfu-remote/current", "livekit\n", nil)
	f.Answer("/access", "mode=allow\naddress=192.168.1.95\nallowed=198.51.100.4/32\n", nil)
	o.Dial = func(addr string) error {
		if strings.HasSuffix(addr, ":7881") {
			return errors.New("refused")
		}
		return nil
	}
	r, err := Status(context.Background(), f, o)
	if err != nil {
		t.Fatal(err)
	}
	out := r.String()
	for _, want := range []string{"host:      fetis@192.168.1.95", "server:    livekit (running)", "access:    allow 198.51.100.4/32", "address:   192.168.1.95",
		"reachable: 7880/tcp ok", "7881/tcp unreachable", "UDP ports cannot be probed: a bot run shows them"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
}

func TestStatus_PrintsNoSecret(t *testing.T) {
	f, o := machine(t, "livekit")
	f.Answer("sfu-remote/current", "livekit\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	f.Answer("/.env", "LIVEKIT_API_SECRET=topsecretvalue\n", nil)
	o.Dial = func(string) error { return nil }
	r, err := Status(context.Background(), f, o)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.String(), "topsecretvalue") {
		t.Errorf("secret printed:\n%s", r)
	}
}

func TestStatus_Nothing(t *testing.T) {
	f, o := machine(t, "janus")
	r, err := Status(context.Background(), f, o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.String(), "server:    none") {
		t.Errorf("%s", r)
	}
}

func TestAllow(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=allow\naddress=203.0.113.7\nallowed=198.51.100.4/32\n", nil)
	if err := Allow(context.Background(), f, prefixes("198.51.100.9/32"), o); err != nil {
		t.Fatal(err)
	}
	if f.index("-A SFU-REMOTE -s 198.51.100.9/32") < 0 {
		t.Errorf("no rule in %v", f.commands())
	}
	if f.index("-I INPUT") >= 0 {
		t.Error("jumps touched")
	}
	if !strings.Contains(string(stdinOf(t, f, "/access")), "allowed=198.51.100.9/32") {
		t.Error("access not updated")
	}
}

func TestAllow_Refused(t *testing.T) {
	f, o := machine(t, "janus")
	if err := Allow(context.Background(), f, prefixes("10.0.0.0/8"), o); err == nil {
		t.Error("nothing deployed: no error")
	}
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	if err := Allow(context.Background(), f, prefixes("10.0.0.0/8"), o); err == nil {
		t.Error("private deployment: no error")
	}
	if f.index("SFU-REMOTE") >= 0 {
		t.Error("rules written")
	}
}

func TestRemove_NothingDeployed(t *testing.T) {
	f, o := machine(t, "janus")
	if err := Remove(context.Background(), f, o); err != nil {
		t.Fatal(err)
	}
	assertNoChange(t, f)
}

func TestRemove_OnlyItsOwn(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	if err := Remove(context.Background(), f, o); err != nil {
		t.Fatal(err)
	}
	down := f.index("docker compose -p sfu-janus down")
	if down < 0 {
		t.Fatalf("no down in %v", f.commands())
	}
	for _, c := range f.commands() {
		for _, bad := range []string{"docker rm", "docker system prune", "docker image", "docker rmi", "--rmi", "--volumes", "iptables"} {
			if strings.Contains(c, bad) {
				t.Errorf("touched more than its own: %s", c)
			}
		}
		if strings.Contains(c, "rm -") && !strings.Contains(c, "sfu-remote") {
			t.Errorf("deleted outside its folder: %s", c)
		}
	}
}

func TestRemove_FilesOwnedByContainers(t *testing.T) {
	f, o := machine(t, "jitsi")
	f.Answer("sfu-remote/current", "jitsi\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	if err := Remove(context.Background(), f, o); err != nil {
		t.Fatal(err)
	}
	i := f.index("docker run --rm")
	if i < 0 {
		t.Fatalf("no throwaway container in %v", f.commands())
	}
	c := f.commands()[i]
	if !strings.Contains(c, "ghcr.io/jitsi/") || !strings.Contains(c, "rm -rf") {
		t.Errorf("%s", c)
	}
}

func TestRemove_Rules(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=allow\naddress=203.0.113.7\nallowed=198.51.100.4/32\n", nil)
	if err := Remove(context.Background(), f, o); err != nil {
		t.Fatal(err)
	}
	down, rules := f.index("compose -p sfu-janus down"), f.index("-X SFU-REMOTE")
	if rules < 0 || rules < down {
		t.Errorf("down %d, rules %d", down, rules)
	}
}

func TestRemove_KeepsMarkerWhenStopFails(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("sfu-remote/current", "janus\n", nil)
	f.Answer("/access", "mode=private\naddress=192.168.1.95\nallowed=\n", nil)
	f.Answer("compose -p sfu-janus down", "", errors.New("no"))
	if err := Remove(context.Background(), f, o); err == nil {
		t.Fatal("no error")
	}
	if f.index("rm -f") >= 0 {
		t.Error("marker removed")
	}
}

// TestRemove_Volumes: images declare anonymous volumes (Jitsi's do), which a
// plain "down" leaves on the machine, one more for each deployment (SC-003).
func TestRemove_Volumes(t *testing.T) {
	f, o := machine(t, "janus")
	f.Answer("sfu-remote/current", "janus\n", nil)
	if err := Remove(context.Background(), f, o); err != nil {
		t.Fatal(err)
	}
	if f.index("compose -p sfu-janus down -v ") < 0 {
		t.Errorf("stop leaves volumes behind: %v", f.commands())
	}
}
