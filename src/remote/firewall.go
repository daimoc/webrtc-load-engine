package remote

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

const chain = "SFU-REMOTE"

// ParseAllowed reads --allow values. The whole internet is refused: it would
// be no restriction at all (FR-008).
func ParseAllowed(values []string) ([]netip.Prefix, error) {
	if len(values) == 0 {
		return nil, errors.New("--allow needs at least one CIDR")
	}
	out := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		p, err := netip.ParsePrefix(v)
		if err != nil {
			return nil, fmt.Errorf("--allow %s: not a CIDR", v)
		}
		if p.Bits() == 0 {
			return nil, fmt.Errorf("--allow %s: this is the whole internet", v)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// firewallBody rebuilds the chain's content and leaves the jumps to it alone,
// so that the allowed addresses can change while a server runs.
func firewallBody(allowed []netip.Prefix, ipv6 bool) []string {
	var out []string
	build := func(tool string, is6 bool) {
		out = append(out,
			"sudo "+tool+" -N "+chain+" 2>/dev/null || true",
			"sudo "+tool+" -F "+chain,
			"sudo "+tool+" -A "+chain+" -m conntrack --ctstate ESTABLISHED,RELATED -j RETURN")
		for _, p := range allowed {
			if p.Addr().Is6() == is6 {
				out = append(out, "sudo "+tool+" -A "+chain+" -s "+p.String()+" -j RETURN")
			}
		}
		out = append(out, "sudo "+tool+" -A "+chain+" -j DROP")
	}
	build("iptables", false)
	if ipv6 {
		build("ip6tables", true)
	}
	return out
}

// jumps gives the rules that send a server's ports to the chain, as "CHAIN
// match" pairs without the action. Only those ports are matched, so SSH is
// never at risk.
func jumps(s Server) []string {
	var out []string
	if s.Published {
		// DNAT has already changed the port in DOCKER-USER.
		for _, p := range s.TCPPorts {
			out = append(out, "DOCKER-USER -p tcp -m conntrack --ctorigdstport "+strconv.Itoa(p))
		}
		for _, r := range s.UDPPorts {
			out = append(out, "DOCKER-USER -p udp -m conntrack --ctorigdstport "+portRange(r))
		}
		return out
	}
	ports := make([]string, len(s.TCPPorts))
	for i, p := range s.TCPPorts {
		ports[i] = strconv.Itoa(p)
	}
	out = append(out, "INPUT -p tcp -m multiport --dports "+strings.Join(ports, ","))
	ranges := make([]string, len(s.UDPPorts))
	for i, r := range s.UDPPorts {
		ranges[i] = portRange(r)
	}
	return append(out, "INPUT -p udp -m multiport --dports "+strings.Join(ranges, ","))
}

func portRange(r PortRange) string {
	if r.First == r.Last {
		return strconv.Itoa(r.First)
	}
	return strconv.Itoa(r.First) + ":" + strconv.Itoa(r.Last)
}

// FirewallCommands installs the chain and its jumps, and is safe to repeat
// (research R4). ipv6 is true when the machine has a public IPv6 address.
func FirewallCommands(s Server, allowed []netip.Prefix, ipv6 bool) []string {
	out := firewallBody(allowed, ipv6)
	jump := func(tool, rule string) string {
		name, match, _ := strings.Cut(rule, " ")
		add := "sudo " + tool + " -C " + name + " " + match + " -j " + chain + " 2>/dev/null || sudo " + tool + " -I " + name + " " + match + " -j " + chain
		if name == "DOCKER-USER" && tool == "ip6tables" {
			// Docker only makes this chain when it handles IPv6.
			return "if sudo ip6tables -S DOCKER-USER >/dev/null 2>&1; then " + add + "; fi"
		}
		return add
	}
	for _, rule := range jumps(s) {
		out = append(out, jump("iptables", rule))
		if ipv6 {
			out = append(out, jump("ip6tables", rule))
		}
	}
	return out
}

// RemoveFirewallCommands deletes the jumps, then the chain, for both address
// families. Each step tolerates having nothing to remove.
func RemoveFirewallCommands(s Server) []string {
	var out []string
	for _, rule := range jumps(s) {
		name, match, _ := strings.Cut(rule, " ")
		for _, tool := range []string{"iptables", "ip6tables"} {
			out = append(out, "sudo "+tool+" -D "+name+" "+match+" -j "+chain+" 2>/dev/null || true")
		}
	}
	for _, tool := range []string{"iptables", "ip6tables"} {
		out = append(out,
			"sudo "+tool+" -F "+chain+" 2>/dev/null || true",
			"sudo "+tool+" -X "+chain+" 2>/dev/null || true")
	}
	return out
}
