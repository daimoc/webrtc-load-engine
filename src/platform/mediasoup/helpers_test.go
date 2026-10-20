package mediasoup

import (
	"bytes"
	"io"
	"log/slog"
	"sync"
)

var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

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
