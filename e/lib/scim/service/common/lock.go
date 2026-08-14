package common

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
)

// Locker is the interface for lock acquisition.
type Locker interface {
	// Acquire obtains a lock for the given key.
	// It returns an unlock function that releases the lock when called.
	Acquire(ctx context.Context, key string) (unlock func(context.Context) error, err error)
}

// DistributedLocker provides distributed locking using Teleport's semaphore service.
type DistributedLocker struct {
	clock clockwork.Clock
	sem   types.Semaphores
}

// checkAndSetDefaults validates the DistributedLocker configuration and sets default values.
func (d *DistributedLocker) checkAndSetDefaults() error {
	if d.sem == nil {
		return trace.BadParameter("semaphores service is required")
	}
	if d.clock == nil {
		d.clock = clockwork.NewRealClock()
	}
	return nil
}

// DistributedLockerOption is a functional option for configuring DistributedLocker.
type DistributedLockerOption func(*DistributedLocker)

// WithClock sets the clock for the distributed locker.
// This is primarily useful for testing with fake clocks.
func WithClock(clock clockwork.Clock) DistributedLockerOption {
	return func(dl *DistributedLocker) {
		dl.clock = clock
	}
}

// NewDistributedLocker creates a new DistributedLocker with the given semaphore service and options.
func NewDistributedLocker(sem types.Semaphores, opts ...DistributedLockerOption) (*DistributedLocker, error) {
	dl := &DistributedLocker{
		clock: clockwork.NewRealClock(),
		sem:   sem,
	}
	for _, opt := range opts {
		opt(dl)
	}
	if err := dl.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return dl, nil
}

// Acquire obtains a distributed semaphore lock for the given key.
func (d *DistributedLocker) Acquire(ctx context.Context, key string) (unlock func(context.Context) error, err error) {
	if key == "" {
		return nil, trace.BadParameter("lock key must not be empty")
	}

	lease, err := services.AcquireSemaphoreWithRetry(ctx, d.config(key))
	if err != nil {
		return nil, trace.Wrap(err, "acquiring distributed semaphore lock for key %q", key)
	}

	return func(releaseCtx context.Context) error {
		return trace.Wrap(d.sem.CancelSemaphoreLease(releaseCtx, *lease), "releasing distributed semaphore lock for key %q", key)
	}, nil
}

// config creates the semaphore configuration for the given lock name.
func (d *DistributedLocker) config(name string) services.AcquireSemaphoreWithRetryConfig {
	return services.AcquireSemaphoreWithRetryConfig{
		Service: d.sem,
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.PluginTypeSCIM,
			SemaphoreName: name,
			MaxLeases:     1,
		},
		Retry: retryutils.LinearConfig{
			Step:   time.Millisecond * 200,
			Max:    2 * time.Minute,
			Jitter: retryutils.HalfJitter,
			Clock:  d.clock,
		},
		TTL: 2 * time.Minute,
	}
}

// SingleProcessLocker provides in-process locking to ensure only one goroutine
// per key can execute at a time. This is not distributed and only prevents
// concurrent operations within the same process.
type SingleProcessLocker struct {
	mu sync.Mutex
	m  map[string]*lockEntry
}

// lockEntry tracks a lock channel and its reference count.
type lockEntry struct {
	ch       chan struct{}
	refCount int
}

func (l *lockEntry) acquire(ctx context.Context) error {
	select {
	case l.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

func (l *lockEntry) release() {
	<-l.ch
}

// NewSingleProcessLocker creates a new SingleProcessLocker.
func NewSingleProcessLocker() *SingleProcessLocker {
	return &SingleProcessLocker{
		m: make(map[string]*lockEntry),
	}
}

// Acquire obtains an in-process lock for the given key.
// It ensures only one goroutine can hold the lock for a specific key at a time
// within the same process.
func (s *SingleProcessLocker) Acquire(ctx context.Context, key string) (unlock func(context.Context) error, err error) {
	if key == "" {
		return nil, trace.BadParameter("lock key must not be empty")
	}

	s.mu.Lock()
	entry, exists := s.m[key]
	if !exists {
		entry = &lockEntry{
			ch:       make(chan struct{}, 1),
			refCount: 0,
		}
		s.m[key] = entry
	}
	entry.refCount++
	s.mu.Unlock()

	cleanup := func() {
		s.mu.Lock()
		entry.refCount--
		if entry.refCount == 0 {
			// No more references to this lock, remove it from the map.
			delete(s.m, key)
		}
		s.mu.Unlock()
	}

	if err := entry.acquire(ctx); err != nil {
		cleanup()
		return nil, trace.Wrap(err)
	}

	return func(_ context.Context) error {
		cleanup()
		entry.release()
		return nil
	}, nil
}

// LockerChain chains multiple lockers together, acquiring them in order.
// This is useful for combining in-process and distributed locking for optimization.
type LockerChain struct {
	lockers []Locker
}

// NewLockerChain creates a new LockerChain with the given lockers.
// Lockers will be acquired in the order they are provided.
func NewLockerChain(lockers ...Locker) *LockerChain {
	return &LockerChain{
		lockers: lockers,
	}
}

// Acquire obtains locks from all lockers in the chain sequentially.
// If any lock acquisition fails, all previously acquired locks are released.
func (c *LockerChain) Acquire(ctx context.Context, key string) (unlock func(context.Context) error, err error) {
	if len(c.lockers) == 0 {
		return nil, trace.BadParameter("no lockers in chain")
	}
	var unlocks []func(context.Context) error
	for _, locker := range c.lockers {
		unlock, err := locker.Acquire(ctx, key)
		if err != nil {
			for _, unlockFn := range slices.Backward(unlocks) {
				// If acquisition fails, release all previously acquired locks in reverse order
				_ = unlockFn(ctx)
			}
			return nil, trace.Wrap(err)
		}
		unlocks = append(unlocks, unlock)
	}

	// Return a combined unlock function that releases all locks in reverse order
	return func(releaseCtx context.Context) error {
		var errs []error
		for _, unlockFn := range slices.Backward(unlocks) {
			if err := unlockFn(releaseCtx); err != nil {
				errs = append(errs, err)
			}
		}
		return trace.NewAggregate(errs...)
	}, nil
}
