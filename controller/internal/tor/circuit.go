package tor

import (
	"fmt"
	"regexp"
	"strings"
)

// circuitIDRe validates a circuit ID (numeric only).
var circuitIDRe = regexp.MustCompile(`^[0-9]+$`)

// CircuitInfo represents a Tor circuit.
type CircuitInfo struct {
	ID      string
	Status  string // LAUNCHED, BUILT, EXTENDED, FAILED, CLOSED
	Path    []string
	Purpose string
}

// GetCircuits retrieves the list of current Tor circuits.
func (c *ControlClient) GetCircuits() ([]CircuitInfo, error) {
	info, err := c.GetInfo("circuit-status")
	if err != nil {
		return nil, err
	}

	raw, ok := info["circuit-status"]
	if !ok {
		return nil, nil
	}

	var circuits []CircuitInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ci := parseCircuitLine(line)
		circuits = append(circuits, ci)
	}
	return circuits, nil
}

// CloseCircuit terminates the circuit with the given numeric ID.
func (c *ControlClient) CloseCircuit(circuitID string) error {
	if !circuitIDRe.MatchString(circuitID) {
		return fmt.Errorf("tor: invalid circuit ID %q", circuitID)
	}
	lines, err := c.sendCommand("CLOSECIRCUIT " + circuitID)
	if err != nil {
		return err
	}
	return expectOK(lines)
}

// parseCircuitLine parses a single circuit-status line.
// Format: ID STATUS PATH PURPOSE=purpose
func parseCircuitLine(line string) CircuitInfo {
	ci := CircuitInfo{}
	fields := strings.Fields(line)
	if len(fields) >= 1 {
		ci.ID = fields[0]
	}
	if len(fields) >= 2 {
		ci.Status = fields[1]
	}
	if len(fields) >= 3 {
		pathField := fields[2]
		// Check if it's a PURPOSE= field (no path present).
		if strings.HasPrefix(pathField, "PURPOSE=") {
			ci.Purpose = pathField[len("PURPOSE="):]
			return ci
		}
		ci.Path = strings.Split(pathField, ",")
	}
	// Look for PURPOSE= in remaining fields.
	for _, f := range fields[3:] {
		if strings.HasPrefix(f, "PURPOSE=") {
			ci.Purpose = f[len("PURPOSE="):]
			break
		}
	}
	return ci
}
