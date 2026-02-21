package vm

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// QMPClient communicates with QEMU via the QMP (QEMU Machine Protocol).
type QMPClient struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
}

type qmpGreeting struct {
	QMP json.RawMessage `json:"QMP"`
}

type qmpCommand struct {
	Execute string `json:"execute"`
}

type qmpResponse struct {
	Return json.RawMessage `json:"return,omitempty"`
	Error  *qmpError       `json:"error,omitempty"`
}

type qmpError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

type qmpStatusResult struct {
	Status  string `json:"status"`
	Running bool   `json:"running"`
}

// NewQMPClient connects to the QMP socket at the given path and
// negotiates capabilities.
func NewQMPClient(socketPath string) (*QMPClient, error) {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("qmp: dial %s: %w", socketPath, err)
	}

	client := &QMPClient{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}

	// Read the QMP greeting.
	var greeting qmpGreeting
	if err := client.decoder.Decode(&greeting); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: read greeting: %w", err)
	}

	// Negotiate capabilities.
	if err := client.execute("qmp_capabilities"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: negotiate capabilities: %w", err)
	}

	return client, nil
}

// SystemPowerdown requests a graceful VM shutdown.
func (c *QMPClient) SystemPowerdown() error {
	return c.execute("system_powerdown")
}

// QueryStatus returns the current VM run state.
func (c *QMPClient) QueryStatus() (string, bool, error) {
	if err := c.encoder.Encode(qmpCommand{Execute: "query-status"}); err != nil {
		return "", false, fmt.Errorf("qmp: send query-status: %w", err)
	}

	var resp qmpResponse
	if err := c.decoder.Decode(&resp); err != nil {
		return "", false, fmt.Errorf("qmp: read response: %w", err)
	}

	if resp.Error != nil {
		return "", false, fmt.Errorf("qmp: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	var status qmpStatusResult
	if err := json.Unmarshal(resp.Return, &status); err != nil {
		return "", false, fmt.Errorf("qmp: parse status: %w", err)
	}

	return status.Status, status.Running, nil
}

// Close closes the QMP connection.
func (c *QMPClient) Close() error {
	return c.conn.Close()
}

func (c *QMPClient) execute(command string) error {
	if err := c.encoder.Encode(qmpCommand{Execute: command}); err != nil {
		return fmt.Errorf("qmp: send %s: %w", command, err)
	}

	var resp qmpResponse
	if err := c.decoder.Decode(&resp); err != nil {
		return fmt.Errorf("qmp: read response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("qmp: %s: %s", resp.Error.Class, resp.Error.Desc)
	}

	return nil
}
