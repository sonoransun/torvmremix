package secwatch

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/user/extorvm/controller/internal/logging"
)

// Monitor reads security events from the virtio-serial Unix socket
// connected to the browser VM's secwatchd daemon.
type Monitor struct {
	socketPath string
	logger     *logging.Logger

	mu        sync.Mutex
	observers []SecurityObserver
	conn      net.Conn
	done      chan struct{}
	events    []SecurityEvent // recent events ring buffer
}

const maxEventHistory = 100

// NewMonitor creates a security event monitor for the given socket path.
func NewMonitor(socketPath string, logger *logging.Logger) *Monitor {
	return &Monitor{
		socketPath: socketPath,
		logger:     logger,
		done:       make(chan struct{}),
	}
}

// OnEvent registers a callback for security events.
func (m *Monitor) OnEvent(fn SecurityObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observers = append(m.observers, fn)
}

// Events returns a copy of recent security events.
func (m *Monitor) Events() []SecurityEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SecurityEvent, len(m.events))
	copy(out, m.events)
	return out
}

// Start begins listening for security events from the virtio-serial socket.
// It retries connection with backoff until Stop is called.
func (m *Monitor) Start() {
	go m.readLoop()
}

// Stop closes the monitor connection and stops the read loop.
func (m *Monitor) Stop() {
	select {
	case <-m.done:
		return
	default:
	}
	close(m.done)

	m.mu.Lock()
	if m.conn != nil {
		m.conn.Close()
	}
	m.mu.Unlock()
}

func (m *Monitor) readLoop() {
	backoff := time.Second
	const maxBackoff = 10 * time.Second

	for {
		select {
		case <-m.done:
			return
		default:
		}

		conn, err := net.DialTimeout("unix", m.socketPath, 5*time.Second)
		if err != nil {
			m.logger.Debug("secwatch: connect %s: %v (retrying in %v)", m.socketPath, err, backoff)
			select {
			case <-time.After(backoff):
			case <-m.done:
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		m.mu.Lock()
		m.conn = conn
		m.mu.Unlock()
		backoff = time.Second

		m.logger.Info("secwatch: connected to %s", m.socketPath)
		m.processConnection(conn)
	}
}

func (m *Monitor) processConnection(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-m.done:
			return
		default:
		}

		var ev SecurityEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			m.logger.Debug("secwatch: parse event: %v", err)
			continue
		}

		m.mu.Lock()
		m.events = append(m.events, ev)
		if len(m.events) > maxEventHistory {
			m.events = m.events[len(m.events)-maxEventHistory:]
		}
		snap := make([]SecurityObserver, len(m.observers))
		copy(snap, m.observers)
		m.mu.Unlock()

		m.logger.Info("secwatch: [%s] %s: %s (remediation=%s)",
			ev.Severity, ev.Type, ev.Detail, ev.Remediation)

		for _, fn := range snap {
			fn(ev)
		}
	}

	m.logger.Debug("secwatch: connection closed")
}
