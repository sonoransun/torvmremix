package lifecycle

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"time"
)

// RetryPolicy defines the retry behavior for a lifecycle state.
type RetryPolicy struct {
	MaxAttempts  int
	BaseDelay    time.Duration
	MaxDelay     time.Duration
	JitterFactor float64
}

// ErrorClass categorizes errors as transient or permanent.
type ErrorClass int

const (
	Transient ErrorClass = iota
	Permanent
)

// ClassifyError determines whether an error is transient (retryable) or permanent.
func ClassifyError(state State, err error) ErrorClass {
	if err == nil {
		return Permanent
	}

	// Context deadline exceeded is transient.
	if err == context.DeadlineExceeded {
		return Transient
	}

	msg := err.Error()

	// Transient error patterns.
	transientPatterns := []string{
		"resource temporarily unavailable",
		"address already in use",
		"connection refused",
		"connection reset",
	}
	for _, pattern := range transientPatterns {
		if strings.Contains(msg, pattern) {
			return Transient
		}
	}

	// Permanent error patterns.
	permanentPatterns := []string{
		"permission denied",
		"file not found",
		"no such file",
		"config validation",
	}
	for _, pattern := range permanentPatterns {
		if strings.Contains(msg, pattern) {
			return Permanent
		}
	}

	// Default to transient for unknown errors.
	return Transient
}

// ShouldRetry determines if a failed state should be retried and the delay before the next attempt.
func ShouldRetry(state State, err error, attempt int, policy *RetryPolicy) (bool, time.Duration) {
	if policy == nil {
		return false, 0
	}
	if attempt >= policy.MaxAttempts {
		return false, 0
	}
	if ClassifyError(state, err) == Permanent {
		return false, 0
	}

	// Exponential backoff: baseDelay * 2^attempt, capped at maxDelay.
	delay := time.Duration(float64(policy.BaseDelay) * math.Pow(2, float64(attempt)))
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	// Apply jitter.
	jitter := 1.0 + policy.JitterFactor*rand.Float64()
	delay = time.Duration(float64(delay) * jitter)

	return true, delay
}

// DefaultRetryPolicy returns the default retry policies for retryable lifecycle states.
func DefaultRetryPolicy() map[State]*RetryPolicy {
	return map[State]*RetryPolicy{
		StateCreateTAP: {
			MaxAttempts:  3,
			BaseDelay:    2 * time.Second,
			MaxDelay:     30 * time.Second,
			JitterFactor: 0.2,
		},
		StateLaunchVM: {
			MaxAttempts:  2,
			BaseDelay:    5 * time.Second,
			MaxDelay:     30 * time.Second,
			JitterFactor: 0.2,
		},
		StateSaveNetwork: {
			MaxAttempts:  2,
			BaseDelay:    1 * time.Second,
			MaxDelay:     10 * time.Second,
			JitterFactor: 0.1,
		},
		StateConfigureTAP: {
			MaxAttempts:  2,
			BaseDelay:    2 * time.Second,
			MaxDelay:     15 * time.Second,
			JitterFactor: 0.2,
		},
		StateFlushDNS: {
			MaxAttempts:  2,
			BaseDelay:    1 * time.Second,
			MaxDelay:     5 * time.Second,
			JitterFactor: 0.1,
		},
	}
}
