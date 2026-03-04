package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Recorder exposes Prometheus metrics for the TorVM lifecycle.
type Recorder struct {
	StateTransitions *prometheus.CounterVec
	BootstrapDuration prometheus.Histogram
	FailsafeActive   prometheus.Gauge
	UptimeSeconds    prometheus.Gauge

	mu        sync.Mutex
	startTime time.Time
	stopUptime chan struct{}
}

// NewRecorder creates and registers all Prometheus metrics.
func NewRecorder(reg prometheus.Registerer) *Recorder {
	r := &Recorder{
		StateTransitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "torvm_state_transitions_total",
			Help: "Total number of lifecycle state transitions.",
		}, []string{"from", "to"}),

		BootstrapDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "torvm_bootstrap_duration_seconds",
			Help:    "Time taken for Tor to bootstrap.",
			Buckets: prometheus.DefBuckets,
		}),

		FailsafeActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "torvm_failsafe_active",
			Help: "Whether the failsafe is currently active (1) or not (0).",
		}),

		UptimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "torvm_uptime_seconds",
			Help: "Seconds since the controller started.",
		}),

		startTime:  time.Now(),
		stopUptime: make(chan struct{}),
	}

	reg.MustRegister(r.StateTransitions)
	reg.MustRegister(r.BootstrapDuration)
	reg.MustRegister(r.FailsafeActive)
	reg.MustRegister(r.UptimeSeconds)

	go r.updateUptime()

	return r
}

// RecordTransition records a state transition.
func (r *Recorder) RecordTransition(from, to string) {
	r.StateTransitions.WithLabelValues(from, to).Inc()
}

// RecordBootstrapDuration records the time taken for Tor to bootstrap.
func (r *Recorder) RecordBootstrapDuration(d time.Duration) {
	r.BootstrapDuration.Observe(d.Seconds())
}

// SetFailsafeActive sets the failsafe gauge.
func (r *Recorder) SetFailsafeActive(active bool) {
	if active {
		r.FailsafeActive.Set(1)
	} else {
		r.FailsafeActive.Set(0)
	}
}

// Stop stops the uptime update goroutine.
func (r *Recorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.stopUptime:
	default:
		close(r.stopUptime)
	}
}

func (r *Recorder) updateUptime() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.UptimeSeconds.Set(time.Since(r.startTime).Seconds())
		case <-r.stopUptime:
			return
		}
	}
}
