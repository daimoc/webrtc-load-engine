package remote

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTarget(t *testing.T) {
	good := map[string]Target{
		"host":           {Host: "host"},
		"my-alias":       {Host: "my-alias"},
		"user@host":      {User: "user", Host: "host"},
		"user@host:2222": {User: "user", Host: "host", Port: "2222"},
		"10.0.0.1:22":    {Host: "10.0.0.1", Port: "22"},
		"[::1]:22":       {Host: "::1", Port: "22"},
		"user@[fd00::1]": {User: "user", Host: "fd00::1"},
	}
	for in, want := range good {
		got, err := ParseTarget(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
		} else if got != want {
			t.Errorf("%q = %+v, want %+v", in, got, want)
		}
	}
	for _, in := range []string{"", "-oProxyCommand=x", "user@", "@host", "user@-x", "host:port", "ho st", "user@host:"} {
		if _, err := ParseTarget(in); err == nil {
			t.Errorf("%q: no error", in)
		}
	}
}

func TestTarget_SSHURL(t *testing.T) {
	for in, want := range map[string]string{
		"host":           "ssh://host",
		"user@host:2222": "ssh://user@host:2222",
		"user@[fd00::1]": "ssh://user@[fd00::1]",
	} {
		tg, err := ParseTarget(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := tg.SSHURL(); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// fakeSSH puts a fake ssh first in PATH: it runs script with $0's arguments
// recorded, one per line, in the returned file.
func fakeSSH(t *testing.T, script string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	body := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> " + argsFile + "; done\n" + script + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestSSHRunner_Arguments(t *testing.T) {
	args := fakeSSH(t, "exit 0")
	r := SSHRunner{Target: Target{User: "u", Host: "h"}}
	if _, err := r.Run(context.Background(), "uptime", nil); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(readArgs(t, args), " ")
	if want := "-o BatchMode=yes -o ConnectTimeout=5 -- u@h uptime"; got != want {
		t.Errorf("args %q, want %q", got, want)
	}
}

func TestSSHRunner_Port(t *testing.T) {
	args := fakeSSH(t, "exit 0")
	r := SSHRunner{Target: Target{Host: "h", Port: "2222"}}
	if _, err := r.Run(context.Background(), "uptime", nil); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(readArgs(t, args), " ")
	if want := "-o BatchMode=yes -o ConnectTimeout=5 -p 2222 -- h uptime"; got != want {
		t.Errorf("args %q, want %q", got, want)
	}
}

func TestSSHRunner_Stdin(t *testing.T) {
	fakeSSH(t, "cat")
	r := SSHRunner{Target: Target{Host: "h"}}
	out, err := r.Run(context.Background(), "x", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Errorf("stdout %q", out)
	}
}

func TestSSHRunner_ErrorHasStderr(t *testing.T) {
	fakeSSH(t, "echo 'no such file' >&2; exit 3")
	r := SSHRunner{Target: Target{Host: "h"}}
	_, err := r.Run(context.Background(), "x", nil)
	if err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Errorf("err = %v", err)
	}
}

func TestSSHRunner_Cancel(t *testing.T) {
	fakeSSH(t, "exec sleep 30")
	r := SSHRunner{Target: Target{Host: "h"}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := r.Run(ctx, "x", nil); err == nil {
		t.Error("no error")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("took %v", d)
	}
}
