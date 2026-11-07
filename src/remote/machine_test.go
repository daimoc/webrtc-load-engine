package remote

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"
)

const goodFacts = `kernel=Linux
arch=x86_64
docker=ok
compose=ok
ip=ok
disk_kb=200000000
mem_kb=19913340
rmem=4194304
wmem=4194304
sudo=no
`

func factsWith(replace ...string) string {
	return strings.NewReplacer(replace...).Replace(goodFacts)
}

func addrJSON(addrs ...string) []byte {
	var b strings.Builder
	b.WriteString(`[{"ifname":"lo","addr_info":[{"family":"inet","local":"127.0.0.1","prefixlen":8,"scope":"host"}]}`)
	for _, a := range addrs {
		fam := "inet"
		if strings.Contains(a, ":") {
			fam = "inet6"
		}
		b.WriteString(`,{"ifname":"eth0","addr_info":[{"family":"` + fam + `","local":"` + a + `","prefixlen":24,"scope":"global"}]}`)
	}
	b.WriteString("]")
	return []byte(b.String())
}

func TestClassify_ValidationMachine(t *testing.T) {
	b, err := os.ReadFile("testdata/ip-addr-private.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := Classify(b)
	if err != nil {
		t.Fatal(err)
	}
	// Its two global IPv6 addresses make it public, whatever IPv4 says.
	if len(a.Public) != 2 || !a.hasPublicIPv6() {
		t.Errorf("public %v", a.Public)
	}
	if got := a.Interfaces[netip.MustParseAddr("192.168.1.95")]; got != "enp2s0f0" {
		t.Errorf("interface %q", got)
	}
	if got := a.Interfaces[netip.MustParseAddr("172.17.0.1")]; got != "docker0" {
		t.Errorf("docker0 %q", got)
	}
}

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name         string
		addrs        []string
		public, priv int
	}{
		{"rfc1918", []string{"10.0.0.1", "172.16.0.1", "192.168.0.1"}, 0, 3},
		{"cgnat vpn", []string{"100.64.0.1"}, 0, 1},
		{"ula", []string{"fd00::1"}, 0, 1},
		{"link-local ignored", []string{"169.254.1.1", "fe80::1"}, 0, 0},
		{"public v4", []string{"203.0.113.7"}, 1, 0},
		{"public v6", []string{"2001:db8::1"}, 1, 0},
		{"vpn plus public", []string{"100.64.0.1", "203.0.113.7"}, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Classify(addrJSON(tc.addrs...))
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Public) != tc.public || len(a.Private) != tc.priv {
				t.Errorf("public %v private %v", a.Public, a.Private)
			}
		})
	}
}

func TestClassify_BadJSON(t *testing.T) {
	if _, err := Classify([]byte("not json")); err == nil {
		t.Error("no error")
	}
}

func checkWith(t *testing.T, server string, facts string, o Options) ([]string, error) {
	t.Helper()
	f := &FakeRunner{}
	f.Answer("uname -m", facts, nil)
	s, _ := LookupServer(server)
	return Check(context.Background(), f, s, o)
}

func TestCheck_Good(t *testing.T) {
	w, err := checkWith(t, "jitsi", goodFacts, Options{})
	if err != nil || len(w) != 0 {
		t.Errorf("warnings %v err %v", w, err)
	}
}

func TestCheck_ReportsEveryFailure(t *testing.T) {
	facts := factsWith("arch=x86_64", "arch=aarch64", "docker=ok", "docker=no", "compose=ok", "compose=no",
		"ip=ok", "ip=no", "disk_kb=200000000", "disk_kb=100", "mem_kb=19913340", "mem_kb=100")
	_, err := checkWith(t, "janus", facts, Options{Allow: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}})
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"aarch64", "docker", "Compose", "disk", "memory", "ip", "sudo iptables"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q not in: %v", want, err)
		}
	}
}

func TestCheck_MemoryFollowsTheServer(t *testing.T) {
	facts := factsWith("mem_kb=19913340", "mem_kb=8388608") // 8 GiB
	if _, err := checkWith(t, "janus", facts, Options{}); err != nil {
		t.Errorf("janus: %v", err)
	}
	if _, err := checkWith(t, "jitsi", facts, Options{}); err == nil {
		t.Error("jitsi: no error")
	}
}

func TestCheck_SudoOnlyWithAllow(t *testing.T) {
	if _, err := checkWith(t, "janus", goodFacts, Options{}); err != nil {
		t.Errorf("no --allow: %v", err)
	}
	o := Options{Allow: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	if _, err := checkWith(t, "janus", factsWith("sudo=no", "sudo=ok"), o); err != nil {
		t.Errorf("sudo ok: %v", err)
	}
}

func TestCheck_BuffersWarnForMediasoupOnly(t *testing.T) {
	for _, facts := range []string{factsWith("rmem=4194304", "rmem=212992"), factsWith("wmem=4194304", "wmem=212992")} {
		w, err := checkWith(t, "mediasoup", facts, Options{})
		if err != nil || len(w) != 1 {
			t.Errorf("mediasoup: %v %v", w, err)
		}
		w, err = checkWith(t, "janus", facts, Options{})
		if err != nil || len(w) != 0 {
			t.Errorf("janus: %v %v", w, err)
		}
	}
}

func TestCheck_Unreachable(t *testing.T) {
	f := &FakeRunner{}
	boom := errors.New("connection refused")
	f.Answer("uname -m", "", boom)
	s, _ := LookupServer("janus")
	if _, err := Check(context.Background(), f, s, Options{}); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}
