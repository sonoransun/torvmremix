package testutil

import (
	"bytes"

	"github.com/user/extorvm/controller/internal/logging"
)

// NewTestLogger creates a Logger that writes to a bytes.Buffer for test assertions.
// The logger is set to LevelDebug so all messages are captured.
func NewTestLogger() (*logging.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger, _ := logging.NewLogger(logging.Options{Verbose: true})
	// Replace the default stderr writer with our buffer.
	// We create a fresh logger manually since NewLogger always adds stderr.
	// Instead, add the buffer as a writer and return it for inspection.
	logger.AddWriter(&buf)
	return logger, &buf
}
