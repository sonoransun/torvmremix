package logging

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"
)

// JSONWriter is an io.Writer that reformats log lines into JSON.
// It expects input in the standard logger format:
//
//	[2006/01/02 15:04:05.000 UTC] LEVEL: message
//
// and emits {"ts":"...","level":"...","msg":"..."} per line.
type JSONWriter struct {
	mu      sync.Mutex
	out     io.Writer
	partial string
}

// NewJSONWriter creates a JSONWriter that writes JSON lines to out.
func NewJSONWriter(out io.Writer) *JSONWriter {
	return &JSONWriter{out: out}
}

type jsonEntry struct {
	Ts    string `json:"ts"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// Write implements io.Writer. It buffers partial lines and emits
// one JSON object per complete line.
func (j *JSONWriter) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	data := j.partial + string(p)
	j.partial = ""

	for {
		idx := strings.IndexByte(data, '\n')
		if idx < 0 {
			j.partial = data
			break
		}
		line := data[:idx]
		data = data[idx+1:]

		entry := parseLine(line)
		b, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		b = append(b, '\n')
		if _, err := j.out.Write(b); err != nil {
			return len(p), err
		}
	}

	return len(p), nil
}

// parseLine extracts timestamp, level, and message from a standard log line.
func parseLine(line string) jsonEntry {
	// Expected format: [2006/01/02 15:04:05.000 UTC] LEVEL: message
	line = strings.TrimSpace(line)

	// Try to parse the bracketed timestamp.
	if len(line) > 0 && line[0] == '[' {
		end := strings.Index(line, "]")
		if end > 0 {
			ts := strings.TrimSpace(line[1:end])
			rest := strings.TrimSpace(line[end+1:])

			// Parse "LEVEL: message"
			if colonIdx := strings.Index(rest, ": "); colonIdx > 0 {
				level := rest[:colonIdx]
				msg := rest[colonIdx+2:]
				return jsonEntry{Ts: ts, Level: level, Msg: msg}
			}
			return jsonEntry{Ts: ts, Level: "INFO", Msg: rest}
		}
	}

	// Fallback: emit the raw line.
	return jsonEntry{
		Ts:    time.Now().UTC().Format(time.RFC3339),
		Level: "INFO",
		Msg:   line,
	}
}
