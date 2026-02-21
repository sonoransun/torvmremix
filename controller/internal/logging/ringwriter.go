package logging

import (
	"strings"
	"sync"
)

// RingWriter is a thread-safe ring buffer that implements io.Writer.
// It stores log lines and optionally calls a callback for each new line.
type RingWriter struct {
	mu       sync.Mutex
	lines    []string
	capacity int
	pos      int
	full     bool
	onLine   func(string)
	partial  string // incomplete line buffer
}

// NewRingWriter creates a RingWriter with the given line capacity.
func NewRingWriter(capacity int) *RingWriter {
	return &RingWriter{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

// Write implements io.Writer. It splits input into lines, stores them
// in the ring buffer, and calls the onLine callback for each complete line.
func (r *RingWriter) Write(p []byte) (int, error) {
	r.mu.Lock()

	data := r.partial + string(p)
	r.partial = ""

	var newLines []string
	for {
		idx := strings.IndexByte(data, '\n')
		if idx < 0 {
			r.partial = data
			break
		}
		line := data[:idx]
		data = data[idx+1:]

		r.lines[r.pos] = line
		r.pos++
		if r.pos >= r.capacity {
			r.pos = 0
			r.full = true
		}
		newLines = append(newLines, line)
	}

	cb := r.onLine
	r.mu.Unlock()

	// Invoke callback outside lock to prevent deadlock.
	if cb != nil {
		for _, line := range newLines {
			cb(line)
		}
	}

	return len(p), nil
}

// Lines returns a copy of all stored lines in chronological order.
func (r *RingWriter) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.full {
		out := make([]string, r.pos)
		copy(out, r.lines[:r.pos])
		return out
	}

	out := make([]string, r.capacity)
	copy(out, r.lines[r.pos:])
	copy(out[r.capacity-r.pos:], r.lines[:r.pos])
	return out
}

// OnLine sets a callback that is invoked for each new complete line.
func (r *RingWriter) OnLine(fn func(string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onLine = fn
}
