package common

import (
	"errors"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/limiter"
)

// RateLimitExceededError is returned by [RateLimiter.CheckRateLimit] when the
// rate limit is exceeded. It exposes the Delay the caller should wait before
// retrying and satisfies [trace.IsLimitExceeded].
type RateLimitExceededError struct {
	// Delay is how long to wait before retrying.
	Delay time.Duration
	err   error
}

func (e *RateLimitExceededError) Error() string { return e.err.Error() }
func (e *RateLimitExceededError) Unwrap() error { return e.err }

// RateLimitConfig defines the rate limit configuration for SCIM operations.
type RateLimitConfig struct {
	Average                 int64
	Burst                   int64
	PeriodSeconds           int64
	MaxConcurrentOperations int64
}

// RateLimiter enforces request rate limits and, optionally, a cap on
// concurrent mutating operations.
type RateLimiter struct {
	rateLimiter *limiter.RateLimiter
	// connLimiter is nil when no concurrency cap is configured.
	connLimiter *limiter.ConnectionsLimiter
}

// NewRateLimiter creates a RateLimiter from the supplied configuration.
func NewRateLimiter(cfg RateLimitConfig) (*RateLimiter, error) {
	rl, err := limiter.NewRateLimiter(limiter.Config{
		Rates: []limiter.Rate{{
			Period:  time.Duration(cfg.PeriodSeconds) * time.Second,
			Average: cfg.Average,
			Burst:   cfg.Burst,
		}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := &RateLimiter{rateLimiter: rl}
	if cfg.MaxConcurrentOperations > 0 {
		m.connLimiter = limiter.NewConnectionsLimiter(cfg.MaxConcurrentOperations)
	}
	return m, nil
}

// CheckRateLimit checks whether the token bucket allows this request. When the
// limit is exceeded it returns a [*RateLimitExceededError].
func (m *RateLimiter) CheckRateLimit(token string) error {
	err := m.rateLimiter.RegisterRequest(token)
	if err == nil {
		return nil
	}
	var rlErr *limiter.RateLimitExceededError
	if errors.As(err, &rlErr) {
		return &RateLimitExceededError{Delay: rlErr.Delay, err: err}
	}
	return trace.Wrap(err)
}

// AcquireMutation acquires a concurrency slot for a mutating operation.
// The caller must invoke the returned release function when the operation completes.
func (m *RateLimiter) AcquireMutation(token string) (release func(), err error) {
	if m.connLimiter == nil {
		return func() {}, nil
	}
	if err := m.connLimiter.AcquireConnection(token); err != nil {
		return func() {}, trace.LimitExceeded("too many concurrent SCIM mutations: %v", err)
	}
	return func() { m.connLimiter.ReleaseConnection(token) }, nil
}
