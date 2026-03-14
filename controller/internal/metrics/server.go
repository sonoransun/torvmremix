package metrics

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HealthStatus is the JSON response for /healthz.
type HealthStatus struct {
	State            string `json:"state"`
	Bootstrap        int    `json:"bootstrap"`
	Failsafe         bool   `json:"failsafe"`
	UptimeSeconds    int    `json:"uptime_seconds"`
	BootstrapPercent int    `json:"tor_bootstrap_percent"`
	VMPID            int    `json:"vm_pid"`
	LastError        string `json:"last_error,omitempty"`
	Version          string `json:"version"`
}

// HealthFunc returns the current health status.
type HealthFunc func() HealthStatus

// Server serves Prometheus metrics and a health endpoint.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
}

// NewServer creates a metrics/health HTTP server on the given address.
func NewServer(addr string, reg *prometheus.Registry, healthFn HealthFunc) (*Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := healthFn()
		json.NewEncoder(w).Encode(status)
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	srv := &http.Server{Handler: mux}
	return &Server{httpServer: srv, listener: ln}, nil
}

// Start begins serving in a goroutine.
func (s *Server) Start() {
	go s.httpServer.Serve(s.listener)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the listener address (useful when port 0 is used).
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}
