package tor

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ControlClient communicates with Tor via the Control Protocol (text-based over TCP).
type ControlClient struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	mu     sync.Mutex
	events chan AsyncEvent
	done   chan struct{}

	// syncResp carries non-async response lines from the reader goroutine
	// to the command methods waiting for a reply.
	syncResp chan string
}

// AsyncEvent represents an asynchronous event from Tor (code 650).
type AsyncEvent struct {
	Code   int
	Action string
	Lines  []string
}

// NewControlClient connects to the Tor Control port at addr and starts
// the background reader goroutine.
func NewControlClient(addr string, timeout time.Duration) (*ControlClient, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("tor: dial %s: %w", addr, err)
	}

	c := &ControlClient{
		conn:     conn,
		reader:   bufio.NewReader(conn),
		writer:   bufio.NewWriter(conn),
		events:   make(chan AsyncEvent, 64),
		done:     make(chan struct{}),
		syncResp: make(chan string, 128),
	}

	go c.readLoop()
	return c, nil
}

// Authenticate sends an AUTHENTICATE command with the given password.
func (c *ControlClient) Authenticate(password string) error {
	if err := validateNoNewlines(password); err != nil {
		return fmt.Errorf("tor: authenticate: %w", err)
	}

	lines, err := c.sendCommand(fmt.Sprintf("AUTHENTICATE \"%s\"", password))
	if err != nil {
		return err
	}
	return expectOK(lines)
}

// GetInfo retrieves values for the given keywords from Tor.
func (c *ControlClient) GetInfo(keywords ...string) (map[string]string, error) {
	for _, kw := range keywords {
		if err := validateNoNewlines(kw); err != nil {
			return nil, fmt.Errorf("tor: getinfo: %w", err)
		}
	}

	lines, err := c.sendCommand("GETINFO " + strings.Join(keywords, " "))
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(keywords))
	for _, line := range lines {
		// Lines are like "250-key=value" or "250 OK"
		body := stripStatusPrefix(line)
		if idx := strings.IndexByte(body, '='); idx >= 0 {
			result[body[:idx]] = body[idx+1:]
		}
	}
	return result, nil
}

// Signal sends a SIGNAL command to Tor (e.g. NEWNYM, SHUTDOWN).
func (c *ControlClient) Signal(sig string) error {
	if err := validateNoNewlines(sig); err != nil {
		return fmt.Errorf("tor: signal: %w", err)
	}

	lines, err := c.sendCommand("SIGNAL " + sig)
	if err != nil {
		return err
	}
	return expectOK(lines)
}

// SetConf sets Tor configuration directives.
func (c *ControlClient) SetConf(directives map[string]string) error {
	var parts []string
	for k, v := range directives {
		if err := validateNoNewlines(k); err != nil {
			return fmt.Errorf("tor: setconf key: %w", err)
		}
		if err := validateNoNewlines(v); err != nil {
			return fmt.Errorf("tor: setconf value: %w", err)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	lines, err := c.sendCommand("SETCONF " + strings.Join(parts, " "))
	if err != nil {
		return err
	}
	return expectOK(lines)
}

// SetEvents subscribes to the given async events (e.g. BW, CIRC, STATUS_CLIENT).
func (c *ControlClient) SetEvents(events []string) error {
	for _, ev := range events {
		if err := validateNoNewlines(ev); err != nil {
			return fmt.Errorf("tor: setevents: %w", err)
		}
	}

	lines, err := c.sendCommand("SETEVENTS " + strings.Join(events, " "))
	if err != nil {
		return err
	}
	return expectOK(lines)
}

// Events returns the channel on which async events (code 650) are delivered.
func (c *ControlClient) Events() <-chan AsyncEvent {
	return c.events
}

// Close closes the connection and stops the reader goroutine.
func (c *ControlClient) Close() error {
	select {
	case <-c.done:
		// Already closed.
		return nil
	default:
	}
	close(c.done)
	return c.conn.Close()
}

// sendCommand sends a command string and collects all response lines.
func (c *ControlClient) sendCommand(cmd string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.writer.WriteString(cmd + "\r\n"); err != nil {
		return nil, fmt.Errorf("tor: write: %w", err)
	}
	if err := c.writer.Flush(); err != nil {
		return nil, fmt.Errorf("tor: flush: %w", err)
	}

	return c.collectResponse()
}

// collectResponse reads response lines from syncResp until we get the
// final line (status code followed by space, not dash).
func (c *ControlClient) collectResponse() ([]string, error) {
	var lines []string
	for {
		select {
		case line, ok := <-c.syncResp:
			if !ok {
				return nil, fmt.Errorf("tor: connection closed")
			}
			lines = append(lines, line)
			// A final response line has a space after the 3-digit code.
			if len(line) >= 4 && line[3] == ' ' {
				// Check for error responses (4xx/5xx).
				if line[0] == '4' || line[0] == '5' {
					return lines, fmt.Errorf("tor: %s", line)
				}
				return lines, nil
			}
		case <-c.done:
			return nil, fmt.Errorf("tor: connection closed")
		}
	}
}

// readLoop runs in a goroutine, reading lines from the connection and
// dispatching them to either the events channel or the syncResp channel.
func (c *ControlClient) readLoop() {
	defer close(c.syncResp)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		line, err := c.reader.ReadString('\n')
		if err != nil {
			select {
			case <-c.done:
			default:
				// Connection error; close down.
				close(c.done)
			}
			return
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		if isAsyncEvent(line) {
			c.dispatchEvent(line)
		} else {
			select {
			case c.syncResp <- line:
			case <-c.done:
				return
			}
		}
	}
}

// dispatchEvent parses a 650 async event line and sends it to the events channel.
func (c *ControlClient) dispatchEvent(line string) {
	// 650 lines: "650 ACTION data" or "650-ACTION data" for multi-line.
	body := ""
	if len(line) > 4 {
		body = line[4:]
	}

	action := body
	if idx := strings.IndexByte(body, ' '); idx >= 0 {
		action = body[:idx]
	}

	ev := AsyncEvent{
		Code:   650,
		Action: action,
		Lines:  []string{body},
	}

	select {
	case c.events <- ev:
	default:
		// Drop event if channel is full.
	}
}

// isAsyncEvent returns true if the line is a 650 asynchronous event.
func isAsyncEvent(line string) bool {
	return len(line) >= 3 && line[:3] == "650"
}

// stripStatusPrefix removes the "NNN-" or "NNN " prefix from a response line.
func stripStatusPrefix(line string) string {
	if len(line) >= 4 {
		return line[4:]
	}
	return line
}

// expectOK checks that the response lines end with a 250 OK.
func expectOK(lines []string) error {
	if len(lines) == 0 {
		return fmt.Errorf("tor: empty response")
	}
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "250 ") {
		return fmt.Errorf("tor: %s", last)
	}
	return nil
}

// validateNoNewlines rejects strings that contain \r or \n to prevent
// protocol injection.
func validateNoNewlines(s string) error {
	if strings.ContainsAny(s, "\r\n") {
		return fmt.Errorf("input contains illegal newline characters")
	}
	return nil
}
