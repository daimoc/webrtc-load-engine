package remote

import (
	"context"
	"io"
	"strings"
	"sync"
)

// fakeAnswer is what FakeRunner answers to commands containing match.
type fakeAnswer struct {
	match string
	out   string
	err   error
}

// fakeCall is one recorded command and what it read on stdin.
type fakeCall struct {
	cmd   string
	stdin []byte
}

// FakeRunner records commands and gives scripted answers: the last answer
// whose match is in the command wins, so a test overrides defaults by
// adding to them.
type FakeRunner struct {
	mu      sync.Mutex
	answers []fakeAnswer
	calls   []fakeCall
}

func (f *FakeRunner) Answer(match, out string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers = append(f.answers, fakeAnswer{match, out, err})
}

func (f *FakeRunner) Run(_ context.Context, cmd string, stdin io.Reader) ([]byte, error) {
	var in []byte
	if stdin != nil {
		in, _ = io.ReadAll(stdin)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{cmd, in})
	for i := len(f.answers) - 1; i >= 0; i-- {
		if strings.Contains(cmd, f.answers[i].match) {
			return []byte(f.answers[i].out), f.answers[i].err
		}
	}
	return nil, nil
}

// commands returns the recorded commands in order.
func (f *FakeRunner) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	for i, c := range f.calls {
		out[i] = c.cmd
	}
	return out
}

// index returns the position of the first command containing s, or -1.
func (f *FakeRunner) index(s string) int {
	for i, c := range f.commands() {
		if strings.Contains(c, s) {
			return i
		}
	}
	return -1
}
