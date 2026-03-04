//go:build !linux

package systemd

import "fmt"

// JournalWriter is a stub for non-Linux platforms.
type JournalWriter struct{}

// NewJournalWriter returns an error on non-Linux platforms since the
// systemd journal is not available.
func NewJournalWriter() (*JournalWriter, error) {
	return nil, fmt.Errorf("systemd: journal not available on this platform")
}

// Write is a stub that always returns an error.
func (w *JournalWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("systemd: journal not available on this platform")
}

// Close is a no-op stub.
func (w *JournalWriter) Close() error {
	return nil
}
