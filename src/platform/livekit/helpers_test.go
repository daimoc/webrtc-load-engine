package livekit

import (
	"bytes"
	"strings"
	"sync"
)

func contains(s, sub string) bool { return strings.Contains(s, sub) }

// lockedBuffer is a log sink written from the adapter's goroutines.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
