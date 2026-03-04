package tor

import (
	"fmt"
	"strconv"
	"strings"
)

// BandwidthStats represents a Tor bandwidth event.
type BandwidthStats struct {
	BytesRead    int64
	BytesWritten int64
}

// ParseBandwidthEvent parses a BW event line.
// Expected format: "BW <read> <written>" (the "650 " prefix should already be stripped).
func ParseBandwidthEvent(line string) (BandwidthStats, error) {
	// Handle both "650 BW ..." and "BW ..." forms.
	s := line
	if strings.HasPrefix(s, "650 ") || strings.HasPrefix(s, "650-") {
		s = s[4:]
	}

	if !strings.HasPrefix(s, "BW ") {
		return BandwidthStats{}, fmt.Errorf("tor: not a BW event: %q", line)
	}

	parts := strings.Fields(s)
	if len(parts) != 3 {
		return BandwidthStats{}, fmt.Errorf("tor: invalid BW event format: %q", line)
	}

	read, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return BandwidthStats{}, fmt.Errorf("tor: invalid bytes read: %w", err)
	}

	written, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return BandwidthStats{}, fmt.Errorf("tor: invalid bytes written: %w", err)
	}

	return BandwidthStats{BytesRead: read, BytesWritten: written}, nil
}
