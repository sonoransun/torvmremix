//go:build linux

package systemd

import (
	"fmt"
	"net"
	"strings"
)

const journalSocket = "/run/systemd/journal/socket"

// JournalWriter implements io.Writer and sends log entries to the systemd
// journal via the native journal protocol socket at /run/systemd/journal/socket.
// Each Write call sends one journal entry with MESSAGE and PRIORITY fields.
type JournalWriter struct {
	conn *net.UnixConn
}

// NewJournalWriter opens a connection to the systemd journal socket.
func NewJournalWriter() (*JournalWriter, error) {
	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{
		Name: journalSocket,
		Net:  "unixgram",
	})
	if err != nil {
		return nil, fmt.Errorf("systemd: dial journal socket: %w", err)
	}
	return &JournalWriter{conn: conn}, nil
}

// Write sends p as a journal MESSAGE with PRIORITY=6 (informational).
// It satisfies the io.Writer interface.
func (w *JournalWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")

	// Journal native protocol: KEY=VALUE pairs separated by newlines.
	// PRIORITY follows syslog levels: 6 = informational.
	entry := "PRIORITY=6\nMESSAGE=" + msg + "\n"

	_, err := w.conn.Write([]byte(entry))
	if err != nil {
		return 0, fmt.Errorf("systemd: write journal: %w", err)
	}
	return len(p), nil
}

// Close closes the underlying connection to the journal socket.
func (w *JournalWriter) Close() error {
	return w.conn.Close()
}
