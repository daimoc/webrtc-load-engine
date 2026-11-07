package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"webrtc-load-engine/src/remote"
)

// noMachine fails any command, so that a test reaching the machine shows.
type noMachine struct{ calls int }

func (n *noMachine) Run(context.Context, string, io.Reader) ([]byte, error) {
	n.calls++
	return nil, io.ErrUnexpectedEOF
}

func runCLI(args ...string) (int, string, *noMachine) {
	n := &noMachine{}
	var out, errOut bytes.Buffer
	code := run(context.Background(), args, &out, &errOut, func(remote.Target) remote.Runner { return n })
	return code, out.String() + errOut.String(), n
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"nonsense"},
		{"deploy"},
		{"deploy", "janus"},
		{"deploy", "nope", "--host", "h"},
		{"deploy", "janus", "--host", "-x"},
		{"deploy", "janus", "--host", "h", "--allow", "0.0.0.0/0"},
		{"deploy", "janus", "--host", "h", "--allow", "::/0"},
		{"deploy", "janus", "--host", "h", "--allow", "10.0.0.1"},
		{"deploy", "janus", "--host", "h", "--unknown"},
		{"status"},
		{"remove"},
		{"allow", "--host", "h"},
		{"allow", "--host", "h", "--allow", "0.0.0.0/0"},
	} {
		code, out, n := runCLI(args...)
		if code != 2 {
			t.Errorf("%v: status %d, output %q", args, code, out)
		}
		if n.calls != 0 {
			t.Errorf("%v: reached the machine", args)
		}
	}
}

func TestFailure(t *testing.T) {
	for _, args := range [][]string{
		{"deploy", "janus", "--host", "h", "--address", "10.0.0.1"},
		{"status", "--host", "h"},
		{"remove", "--host", "h"},
		{"allow", "--host", "h", "--allow", "10.0.0.0/8"},
	} {
		code, out, n := runCLI(args...)
		if code != 1 {
			t.Errorf("%v: status %d, output %q", args, code, out)
		}
		if n.calls == 0 {
			t.Errorf("%v: did not reach the machine", args)
		}
	}
}

func TestRepeatedAllow(t *testing.T) {
	code, out, _ := runCLI("deploy", "janus", "--host", "h", "--allow", "10.0.0.0/8", "--allow", "192.168.0.0/16", "--address", "10.0.0.1")
	if code != 1 || strings.Contains(out, "flag") {
		t.Errorf("status %d, output %q", code, out)
	}
}

type answerRunner struct{}

func (answerRunner) Run(context.Context, string, io.Reader) ([]byte, error) { return nil, nil }

func TestSuccess(t *testing.T) {
	var out bytes.Buffer
	code := run(context.Background(), []string{"remove", "--host", "h"}, &out, &out, func(remote.Target) remote.Runner { return answerRunner{} })
	if code != 0 {
		t.Errorf("status %d, output %q", code, out.String())
	}
	out.Reset()
	code = run(context.Background(), []string{"status", "--host", "h"}, &out, &out, func(remote.Target) remote.Runner { return answerRunner{} })
	if code != 0 || !strings.Contains(out.String(), "none") {
		t.Errorf("status %d, output %q", code, out.String())
	}
}
