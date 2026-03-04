package tor

import (
	"fmt"
	"strconv"
	"strings"
)

// BootstrapStatus represents Tor's bootstrap progress.
type BootstrapStatus struct {
	Progress int
	Tag      string
	Summary  string
}

// ParseBootstrapStatus parses a NOTICE BOOTSTRAP line from Tor.
// Expected format: NOTICE BOOTSTRAP PROGRESS=50 TAG=loading_descriptors SUMMARY="Loading relay descriptors"
func ParseBootstrapStatus(line string) (BootstrapStatus, error) {
	var status BootstrapStatus

	progressIdx := strings.Index(line, "PROGRESS=")
	if progressIdx < 0 {
		return status, fmt.Errorf("tor: missing PROGRESS in bootstrap status: %q", line)
	}
	progressStr := line[progressIdx+len("PROGRESS="):]
	if spaceIdx := strings.IndexByte(progressStr, ' '); spaceIdx >= 0 {
		progressStr = progressStr[:spaceIdx]
	}
	progress, err := strconv.Atoi(progressStr)
	if err != nil {
		return status, fmt.Errorf("tor: invalid PROGRESS value: %w", err)
	}
	status.Progress = progress

	tagIdx := strings.Index(line, "TAG=")
	if tagIdx >= 0 {
		tagStr := line[tagIdx+len("TAG="):]
		if spaceIdx := strings.IndexByte(tagStr, ' '); spaceIdx >= 0 {
			tagStr = tagStr[:spaceIdx]
		}
		status.Tag = tagStr
	}

	summaryIdx := strings.Index(line, "SUMMARY=\"")
	if summaryIdx >= 0 {
		summaryStr := line[summaryIdx+len("SUMMARY=\""):]
		if endQuote := strings.IndexByte(summaryStr, '"'); endQuote >= 0 {
			summaryStr = summaryStr[:endQuote]
		}
		status.Summary = summaryStr
	}

	return status, nil
}

// GetBootstrapStatus queries Tor for the current bootstrap phase.
func (c *ControlClient) GetBootstrapStatus() (BootstrapStatus, error) {
	info, err := c.GetInfo("status/bootstrap-phase")
	if err != nil {
		return BootstrapStatus{}, err
	}

	phase, ok := info["status/bootstrap-phase"]
	if !ok {
		return BootstrapStatus{}, fmt.Errorf("tor: no bootstrap-phase in response")
	}

	return ParseBootstrapStatus(phase)
}
