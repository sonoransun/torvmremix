package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateEntropy returns cryptographically secure random bytes.
func GenerateEntropy(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("entropy: %w", err)
	}
	return buf, nil
}

// EntropyHexString returns a hex-encoded string of n random bytes,
// suitable for the kernel command line ENTROPY= parameter.
func EntropyHexString(n int) (string, error) {
	b, err := GenerateEntropy(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
