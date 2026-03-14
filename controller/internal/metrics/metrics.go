package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Recorder exposes Prometheus metrics for the TorVM lifecycle.
type Recorder struct {
	StateTransitions     *prometheus.CounterVec
	BootstrapDuration    prometheus.Histogram
	FailsafeActive       prometheus.Gauge
	UptimeSeconds        prometheus.Gauge
	BootstrapProgress    prometheus.Gauge
	StateDuration        *prometheus.HistogramVec
	RestartsTotal        prometheus.Counter
	FailsafeActivations prometheus.Counter

	mu              sync.Mutex
	startTime       time.Time
	stopUptime      chan struct{}
	stateEnteredAt  time.Time
	currentState    string
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

		BootstrapProgress: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "torvm_bootstrap_progress",
			Help: "Tor bootstrap percentage (0-100).",
		}),

		StateDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "torvm_state_duration_seconds",
			Help:    "Time spent in each lifecycle state.",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120, 300},
		}, []string{"state"}),

		RestartsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "torvm_restarts_total",
			Help: "Total number of VM restarts.",
		}),

		FailsafeActivations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "torvm_failsafe_activations_total",
			Help: "Total number of failsafe activations.",
		}),

		startTime:      time.Now(),
		stopUptime:     make(chan struct{}),
		stateEnteredAt: time.Now(),
		currentState:   "Init",
	}

	reg.MustRegister(r.StateTransitions)
	reg.MustRegister(r.BootstrapDuration)
	reg.MustRegister(r.FailsafeActive)
	reg.MustRegister(r.UptimeSeconds)
	reg.MustRegister(r.BootstrapProgress)
	reg.MustRegister(r.StateDuration)
	reg.MustRegister(r.RestartsTotal)
	reg.MustRegister(r.FailsafeActivations)

	go r.updateUptime()

	return r
}

// RecordTransition records a state transition and tracks per-state duration.
func (r *Recorder) RecordTransition(from, to string) {
	r.StateTransitions.WithLabelValues(from, to).Inc()

	r.mu.Lock()
	if r.currentState != "" {
		elapsed := time.Since(r.stateEnteredAt).Seconds()
		r.StateDuration.WithLabelValues(r.currentState).Observe(elapsed)
	}
	r.currentState = to
	r.stateEnteredAt = time.Now()
	r.mu.Unlock()

	// Track restarts: if transitioning back to Init or LaunchVM from a later state.
	if to == "LaunchVM" && (from == "Shutdown" || from == "RestoreNetwork" || from == "Cleanup") {
		r.RestartsTotal.Inc()
	}
}

// RecordBootstrapProgress updates the bootstrap progress gauge.
func (r *Recorder) RecordBootstrapProgress(pct int) {
	r.BootstrapProgress.Set(float64(pct))
}

// RecordFailsafeActivation increments the failsafe activation counter.
func (r *Recorder) RecordFailsafeActivation() {
	r.FailsafeActivations.Inc()
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
