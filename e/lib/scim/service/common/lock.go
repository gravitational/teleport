package common

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
)

// SemaphoreLocker provides distributed locking using Teleport's semaphore service.
type SemaphoreLocker struct {
	clock clockwork.Clock
	sem   types.Semaphores
}

// checkAndSetDefaults validates the SemaphoreLocker configuration and sets default values.
func (s *SemaphoreLocker) checkAndSetDefaults() error {
	if s.sem == nil {
		return trace.BadParameter("semaphores service is required")
	}
	if s.clock == nil {
		s.clock = clockwork.NewRealClock()
	}
	return nil
}

// SemaphoreLockerOption is a functional option for configuring SemaphoreLocker.
type SemaphoreLockerOption func(*SemaphoreLocker)

// WithClock sets the clock for the semaphore locker.
// This is primarily useful for testing with fake clocks.
func WithClock(clock clockwork.Clock) SemaphoreLockerOption {
	return func(sl *SemaphoreLocker) {
		sl.clock = clock
	}
}

// NewSemaphoreLocker creates a new SemaphoreLocker with the given semaphore service and options.
func NewSemaphoreLocker(sem types.Semaphores, opts ...SemaphoreLockerOption) (*SemaphoreLocker, error) {
	sl := &SemaphoreLocker{
		clock: clockwork.NewRealClock(),
		sem:   sem,
	}
	for _, opt := range opts {
		opt(sl)
	}
	if err := sl.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return sl, nil
}

// Acquire obtains a distributed semaphore lock for the given key.
func (s *SemaphoreLocker) Acquire(ctx context.Context, key string) (unlock func(context.Context) error, err error) {
	if key == "" {
		return nil, trace.BadParameter("lock key must not be empty")
	}

	lease, err := services.AcquireSemaphoreWithRetry(ctx, s.config(key))
	if err != nil {
		return nil, trace.Wrap(err, "acquiring semaphore lock for key %q", key)
	}

	return func(releaseCtx context.Context) error {
		return trace.Wrap(s.sem.CancelSemaphoreLease(releaseCtx, *lease), "releasing semaphore lock for key %q", key)
	}, nil
}

// config creates the semaphore configuration for the given lock name.
func (s *SemaphoreLocker) config(name string) services.AcquireSemaphoreWithRetryConfig {
	return services.AcquireSemaphoreWithRetryConfig{
		Service: s.sem,
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.PluginTypeSCIM,
			SemaphoreName: name,
			MaxLeases:     1,
		},
		Retry: retryutils.LinearConfig{
			Step:   time.Millisecond * 200,
			Max:    2 * time.Minute,
			Jitter: retryutils.HalfJitter,
			Clock:  s.clock,
		},
		TTL: 2 * time.Minute,
	}
}
