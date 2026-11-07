// Package remote deploys the media servers of tests/platforms to a Linux
// machine over the operator's SSH access (feature 013).
package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Runner runs a shell command on the machine. Secrets go through stdin,
// never cmd: a command line shows in the machine's ps.
type Runner interface {
	Run(ctx context.Context, cmd string, stdin io.Reader) (stdout []byte, err error)
}

// Target is how the operator reaches the machine.
type Target struct {
	User, Host, Port string
}

// ParseTarget reads [user@]host[:port], with an IPv6 host in brackets, or an
// ~/.ssh/config alias.
func ParseTarget(s string) (Target, error) {
	bad := func(why string) (Target, error) {
		return Target{}, fmt.Errorf("target %q: %s", s, why)
	}
	if s == "" {
		return bad("empty")
	}
	if strings.HasPrefix(s, "-") {
		return bad("must not start with -")
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return bad("contains a space")
	}
	var t Target
	rest := s
	if user, after, ok := strings.Cut(s, "@"); ok {
		if user == "" {
			return bad("empty user")
		}
		t.User, rest = user, after
	}
	switch {
	case strings.HasPrefix(rest, "["):
		host, after, ok := strings.Cut(rest[1:], "]")
		if !ok || host == "" {
			return bad("unclosed bracket")
		}
		t.Host = host
		if after != "" {
			port, ok := strings.CutPrefix(after, ":")
			if !ok {
				return bad("text after the bracket")
			}
			t.Port = port
		} else {
			return finishTarget(t, s, false)
		}
	case strings.Count(rest, ":") == 1:
		host, port, _ := strings.Cut(rest, ":")
		t.Host, t.Port = host, port
	default:
		t.Host = rest
		return finishTarget(t, s, false)
	}
	return finishTarget(t, s, true)
}

func finishTarget(t Target, s string, wantPort bool) (Target, error) {
	if t.Host == "" || strings.HasPrefix(t.Host, "-") {
		return Target{}, fmt.Errorf("target %q: bad host", s)
	}
	if wantPort {
		if t.Port == "" || strings.Trim(t.Port, "0123456789") != "" {
			return Target{}, fmt.Errorf("target %q: bad port", s)
		}
	}
	return t, nil
}

// String gives the target as the operator writes it.
func (t Target) String() string {
	s := t.hostPort()
	if t.User != "" {
		s = t.User + "@" + s
	}
	return s
}

func (t Target) hostPort() string {
	host := t.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if t.Port != "" {
		host = net.JoinHostPort(t.Host, t.Port)
	}
	return host
}

// SSHURL gives the DOCKER_HOST value for this target.
func (t Target) SSHURL() string { return "ssh://" + t.String() }

// args gives ssh's arguments up to and including the destination.
func (t Target) args() []string {
	a := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5"}
	if t.Port != "" {
		a = append(a, "-p", t.Port)
	}
	dest := t.Host
	if t.User != "" {
		dest = t.User + "@" + dest
	}
	return append(a, "--", dest)
}

// stderrLimit bounds what a failed command adds to its error: a failed image
// build prints thousands of lines, and the end says why.
const stderrLimit = 4096

// SSHRunner runs commands with the operator's ssh, so that keys, agent,
// ~/.ssh/config and known hosts apply and no credentials enter this project
// (research R1). BatchMode makes a failure an error rather than a prompt.
type SSHRunner struct {
	Target Target
}

// Run implements Runner.
func (r SSHRunner) Run(ctx context.Context, cmd string, stdin io.Reader) ([]byte, error) {
	c := exec.CommandContext(ctx, "ssh", append(r.Target.args(), cmd)...)
	c.Stdin = stdin
	c.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		var exit *exec.ExitError
		if msg := tail(stderr.String(), stderrLimit); errors.As(err, &exit) && msg != "" {
			return stdout.Bytes(), fmt.Errorf("%s: %s", err, msg)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "…" + s[len(s)-n:]
	}
	return s
}
