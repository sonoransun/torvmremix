package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// EntropyHexString returns a hex-encoded string of n random bytes,
// suitable for the kernel command line ENTROPY= parameter.
func EntropyHexString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("entropy: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
