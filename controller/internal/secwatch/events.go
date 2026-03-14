package secwatch

import "time"

// SecurityEvent represents a security event reported by the in-VM
// security monitor daemon (secwatchd) via the virtio-serial channel.
type SecurityEvent struct {
	Type        string `json:"type"`                  // canary_violation, honey_token_access, seccomp_kill, file_tamper
	Severity    string `json:"severity"`              // critical, warning, info
	Detail      string `json:"detail"`
	PID         int    `json:"pid,omitempty"`
	Remediation string `json:"remediation,omitempty"` // killed, isolated, restarted, none
	Timestamp   int64  `json:"ts"`
}

// Time returns the event timestamp as a time.Time.
func (e SecurityEvent) Time() time.Time {
	return time.Unix(e.Timestamp, 0)
}

// IsCritical returns true if the event severity is "critical".
func (e SecurityEvent) IsCritical() bool {
	return e.Severity == "critical"
}

// SecurityObserver is called when a security event is received.
type SecurityObserver func(SecurityEvent)
