package network

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/user/extorvm/controller/internal/logging"
)

// DriftEvent describes a detected route or DNS anomaly.
type DriftEvent struct {
	Type        string // "route_drift" or "dns_leak"
	Description string
}

// VerifierConfig controls the route verifier behaviour.
type VerifierConfig struct {
	// Interval between verification checks. Defaults to 30s if zero.
	Interval time.Duration

	// ExpectedTAP is the name of the TAP device that should carry traffic.
	ExpectedTAP string

	// ExpectedVMIP is the VM gateway IP that routes should point to.
	ExpectedVMIP net.IP
}

// RouteVerifier periodically checks that host routes and DNS still point
// through the TorVM, emitting DriftEvent values when anomalies are found.
//
// Lifecycle integration: start the verifier when the engine enters
// StateRunning and cancel its context on shutdown. Example:
//
//	cfg := network.VerifierConfig{
//	    ExpectedTAP:  "torvm0",
//	    ExpectedVMIP: net.ParseIP("10.10.10.1"),
//	}
//	events := verifier.Start(ctx, cfg)
//	go func() {
//	    for ev := range events {
//	        logger.Error("leak detected: %s: %s", ev.Type, ev.Description)
//	    }
//	}()
type RouteVerifier struct {
	logger *logging.Logger
}

// NewRouteVerifier creates a verifier that logs to the given logger.
func NewRouteVerifier(logger *logging.Logger) *RouteVerifier {
	return &RouteVerifier{logger: logger}
}

// Start begins periodic route and DNS verification. It returns a channel
// that emits DriftEvent values whenever an anomaly is detected. The channel
// is closed when ctx is cancelled.
func (v *RouteVerifier) Start(ctx context.Context, cfg VerifierConfig) <-chan DriftEvent {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}

	ch := make(chan DriftEvent, 4)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		// Run an initial check immediately.
		v.check(ctx, cfg, ch)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				v.check(ctx, cfg, ch)
			}
		}
	}()

	return ch
}

func (v *RouteVerifier) check(ctx context.Context, cfg VerifierConfig, ch chan<- DriftEvent) {
	// Platform-specific route verification.
	if err := verifyRoutes(cfg.ExpectedTAP, cfg.ExpectedVMIP); err != nil {
		v.logger.Error("route drift detected: %v", err)
		select {
		case ch <- DriftEvent{Type: "route_drift", Description: err.Error()}:
		case <-ctx.Done():
			return
		}
	}

	// DNS leak test: attempt a raw UDP connection to 8.8.8.8:53.
	// Under correct routing all DNS goes through Tor's DNSPort, so a
	// direct UDP dial should time out. If it succeeds, DNS may be leaking.
	if err := checkDNSLeak(); err != nil {
		v.logger.Error("dns leak detected: %v", err)
		select {
		case ch <- DriftEvent{Type: "dns_leak", Description: err.Error()}:
		case <-ctx.Done():
			return
		}
	}
}

// checkDNSLeak attempts a raw UDP connection to 8.8.8.8:53. If the
// connection succeeds and bytes can be written, DNS traffic may be
// bypassing Tor.
func checkDNSLeak() error {
	conn, err := net.DialTimeout("udp", "8.8.8.8:53", 1*time.Second)
	if err != nil {
		// Connection refused or timed out — expected under correct routing.
		return nil
	}
	defer conn.Close()

	// Set a short deadline and attempt to write a minimal DNS query.
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))

	// Minimal DNS query for "." (root) — 12-byte header + 5-byte question.
	query := []byte{
		0x00, 0x01, // ID
		0x01, 0x00, // Flags: standard query
		0x00, 0x01, // Questions: 1
		0x00, 0x00, // Answers: 0
		0x00, 0x00, // Authority: 0
		0x00, 0x00, // Additional: 0
		0x00,                   // Root label
		0x00, 0x01, // Type: A
		0x00, 0x01, // Class: IN
	}

	if _, err := conn.Write(query); err != nil {
		return nil
	}

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		// Read timed out or failed — no leak.
		return nil
	}
	if n > 0 {
		return fmt.Errorf("received %d-byte DNS response from 8.8.8.8 outside Tor", n)
	}
	return nil
}
