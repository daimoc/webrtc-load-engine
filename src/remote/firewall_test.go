package remote

import (
	"net/netip"
	"strings"
	"testing"
)

func prefixes(s ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(s))
	for i, p := range s {
		out[i] = netip.MustParsePrefix(p)
	}
	return out
}

func TestParseAllowed(t *testing.T) {
	got, err := ParseAllowed([]string{"198.51.100.4/32", "2001:db8::/32"})
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	for _, bad := range []string{"0.0.0.0/0", "::/0", "198.51.100.4", "nonsense"} {
		if _, err := ParseAllowed([]string{"10.0.0.0/8", bad}); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
	if _, err := ParseAllowed(nil); err == nil {
		t.Error("empty: no error")
	}
}

func TestFirewallCommands_HostNetwork(t *testing.T) {
	s, _ := LookupServer("janus")
	got := strings.Join(FirewallCommands(s, prefixes("198.51.100.4/32", "203.0.113.0/24"), false), "\n")
	est := strings.Index(got, "--ctstate ESTABLISHED,RELATED -j RETURN")
	a := strings.Index(got, "-A SFU-REMOTE -s 198.51.100.4/32 -j RETURN")
	b := strings.Index(got, "-A SFU-REMOTE -s 203.0.113.0/24 -j RETURN")
	drop := strings.Index(got, "-A SFU-REMOTE -j DROP")
	if est < 0 || a < est || b < a || drop < b {
		t.Errorf("order wrong:\n%s", got)
	}
	for _, want := range []string{
		"sudo iptables -I INPUT -p tcp -m multiport --dports 8188 -j SFU-REMOTE",
		"sudo iptables -I INPUT -p udp -m multiport --dports 20000:20999 -j SFU-REMOTE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "DOCKER-USER") || strings.Contains(got, "ip6tables") {
		t.Errorf("unexpected rule:\n%s", got)
	}
}

func TestFirewallCommands_PublishedPorts(t *testing.T) {
	s, _ := LookupServer("jitsi")
	got := strings.Join(FirewallCommands(s, prefixes("198.51.100.4/32"), false), "\n")
	for _, want := range []string{
		"-I DOCKER-USER -p tcp -m conntrack --ctorigdstport 8000 -j SFU-REMOTE",
		"-I DOCKER-USER -p tcp -m conntrack --ctorigdstport 8443 -j SFU-REMOTE",
		"-I DOCKER-USER -p udp -m conntrack --ctorigdstport 10000 -j SFU-REMOTE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "-I INPUT") {
		t.Errorf("INPUT rule for a published server:\n%s", got)
	}
}

func TestFirewallCommands_SSHNeverMatched(t *testing.T) {
	for _, name := range []string{"jitsi", "livekit", "mediasoup", "janus"} {
		s, _ := LookupServer(name)
		for _, c := range FirewallCommands(s, prefixes("10.0.0.0/8"), true) {
			for _, f := range strings.Fields(c) {
				if f == "22" {
					t.Errorf("%s: %s", name, c)
				}
			}
			if strings.Contains(c, "-A INPUT") || strings.Contains(c, "-P ") {
				t.Errorf("%s: changes INPUT outside the chain: %s", name, c)
			}
		}
	}
}

func TestFirewallCommands_IPv6(t *testing.T) {
	s, _ := LookupServer("janus")
	v4 := strings.Join(FirewallCommands(s, prefixes("198.51.100.4/32"), false), "\n")
	if strings.Contains(v4, "ip6tables") {
		t.Errorf("ip6tables without a public IPv6 address:\n%s", v4)
	}
	both := strings.Join(FirewallCommands(s, prefixes("198.51.100.4/32", "2001:db8::/32"), true), "\n")
	for _, want := range []string{
		"sudo ip6tables -A SFU-REMOTE -s 2001:db8::/32 -j RETURN",
		"sudo ip6tables -A SFU-REMOTE -j DROP",
		"sudo iptables -A SFU-REMOTE -s 198.51.100.4/32 -j RETURN",
	} {
		if !strings.Contains(both, want) {
			t.Errorf("no %q in:\n%s", want, both)
		}
	}
	if strings.Contains(both, "ip6tables -A SFU-REMOTE -s 198.51.100.4") || strings.Contains(both, "iptables -A SFU-REMOTE -s 2001") && !strings.Contains(both, "ip6tables -A SFU-REMOTE -s 2001") {
		t.Errorf("address in the wrong family:\n%s", both)
	}
}

func TestFirewallBody_LeavesJumpsAlone(t *testing.T) {
	got := strings.Join(firewallBody(prefixes("198.51.100.4/32"), false), "\n")
	if strings.Contains(got, "INPUT") || strings.Contains(got, "DOCKER-USER") {
		t.Errorf("body touches jumps:\n%s", got)
	}
	if !strings.Contains(got, "-F SFU-REMOTE") {
		t.Errorf("body does not flush:\n%s", got)
	}
}

func TestFirewallCommands_SafeToRepeat(t *testing.T) {
	s, _ := LookupServer("janus")
	got := strings.Join(FirewallCommands(s, prefixes("10.0.0.0/8"), false), "\n")
	if !strings.Contains(got, "sudo iptables -C INPUT -p tcp -m multiport --dports 8188 -j SFU-REMOTE 2>/dev/null || sudo iptables -I INPUT") {
		t.Errorf("jump not idempotent:\n%s", got)
	}
}

func TestRemoveFirewallCommands(t *testing.T) {
	s, _ := LookupServer("jitsi")
	got := RemoveFirewallCommands(s)
	all := strings.Join(got, "\n")
	jump := strings.Index(all, "-D DOCKER-USER")
	flush := strings.Index(all, "iptables -F SFU-REMOTE")
	del := strings.Index(all, "iptables -X SFU-REMOTE")
	if jump < 0 || flush < jump || del < flush {
		t.Errorf("order wrong:\n%s", all)
	}
	if !strings.Contains(all, "ip6tables -X SFU-REMOTE") {
		t.Errorf("no ip6tables:\n%s", all)
	}
}
