package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

const (
	diskNeededKB   = 10 * 1024 * 1024 // Janus and mediasoup build from source
	socketBufferOK = 4 << 20          // what MEDIASOUP_SOCKET_BUFFER asks for
)

// cgnat is 100.64.0.0/10, used by VPNs such as Tailscale.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// Addresses are the machine's own addresses (research R3).
type Addresses struct {
	Private, Public []netip.Addr
	// Interfaces names the interface of each address.
	Interfaces map[netip.Addr]string
}

// Classify reads the output of ip -j addr. Loopback and link-local addresses
// are ignored. Servers listen on every interface, so one public address
// exposes them, whatever the announced address is.
func Classify(ipJSON []byte) (Addresses, error) {
	var links []struct {
		IfName   string `json:"ifname"`
		AddrInfo []struct {
			Local string `json:"local"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(ipJSON, &links); err != nil {
		return Addresses{}, fmt.Errorf("reading the machine's addresses: %w", err)
	}
	a := Addresses{Interfaces: map[netip.Addr]string{}}
	for _, l := range links {
		for _, info := range l.AddrInfo {
			addr, err := netip.ParseAddr(info.Local)
			if err != nil {
				return Addresses{}, fmt.Errorf("reading the machine's addresses: %w", err)
			}
			addr = addr.Unmap()
			if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
				continue
			}
			a.Interfaces[addr] = l.IfName
			if addr.IsPrivate() || cgnat.Contains(addr) {
				a.Private = append(a.Private, addr)
			} else {
				a.Public = append(a.Public, addr)
			}
		}
	}
	return a, nil
}

// hasPublicIPv6 says whether ip6tables rules are needed.
func (a Addresses) hasPublicIPv6() bool {
	for _, p := range a.Public {
		if p.Is6() {
			return true
		}
	}
	return false
}

const factsScript = `echo kernel=$(uname -s); echo arch=$(uname -m)
docker ps >/dev/null 2>&1 && echo docker=ok || echo docker=no
docker compose version >/dev/null 2>&1 && echo compose=ok || echo compose=no
command -v ip >/dev/null 2>&1 && echo ip=ok || echo ip=no
echo disk_kb=$(df -Pk "$HOME" | awk 'NR==2{print $4}')
echo mem_kb=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
echo rmem=$(cat /proc/sys/net/core/rmem_max)
echo wmem=$(cat /proc/sys/net/core/wmem_max)`

const sudoScript = `
sudo -n iptables -S >/dev/null 2>&1 && echo sudo=ok || echo sudo=no`

// Check verifies the machine's prerequisites without changing anything
// (research R7). Every failure is in the returned error, and warnings do not
// stop a deployment.
func Check(ctx context.Context, r Runner, s Server, o Options) (warnings []string, err error) {
	script := factsScript
	if len(o.Allow) > 0 {
		script += sudoScript
	}
	out, err := r.Run(ctx, script, nil)
	if err != nil {
		return nil, fmt.Errorf("reading the machine: %w", err)
	}
	f := ParseEnv(out)
	number := func(key string) int64 {
		n, _ := strconv.ParseInt(f[key], 10, 64)
		return n
	}
	var problems []string
	if f["kernel"] != "Linux" {
		problems = append(problems, fmt.Sprintf("the system is %q, Linux is needed", f["kernel"]))
	}
	if f["arch"] != "x86_64" {
		problems = append(problems, fmt.Sprintf("the architecture is %q, x86_64 is needed", f["arch"]))
	}
	if f["docker"] != "ok" {
		problems = append(problems, "docker does not work without sudo: install it, and add the user to the docker group")
	}
	if f["compose"] != "ok" {
		problems = append(problems, "the Compose plugin is missing")
	}
	if f["ip"] != "ok" {
		problems = append(problems, "the ip command is missing")
	}
	if number("disk_kb") < diskNeededKB {
		problems = append(problems, fmt.Sprintf("%d GB of disk is free, 10 GB is needed", number("disk_kb")/(1024*1024)))
	}
	if number("mem_kb") < int64(s.MemoryGB)*1024*1024 {
		problems = append(problems, fmt.Sprintf("%s needs %d GB of memory, the machine has %d", s.Name, s.MemoryGB, number("mem_kb")/(1024*1024)))
	}
	if len(o.Allow) > 0 && f["sudo"] != "ok" {
		problems = append(problems, "--allow needs passwordless sudo iptables")
	}
	if len(problems) > 0 {
		return nil, errors.New("the machine is not ready:\n  " + strings.Join(problems, "\n  "))
	}
	if s.Name == "mediasoup" {
		for _, key := range []string{"rmem", "wmem"} {
			if number(key) < socketBufferOK {
				warnings = append(warnings, fmt.Sprintf("net.core.%s_max is %d, under 4 MiB: it caps mediasoup's socket buffers", key, number(key)))
			}
		}
	}
	return warnings, nil
}
