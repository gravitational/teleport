/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

type SemaphoreLockConfig struct {
	// Service is the service against which all semaphore
	// operations are performed.
	Service types.Semaphores
	// Expiry is an optional lease expiry parameter.
	Expiry time.Duration
	// TickRate is the rate at which lease renewals are attempted
	// and defaults to 1/2 expiry.  Used to accelerate tests.
	TickRate time.Duration
	// Params holds the semaphore lease acquisition parameters.
	Params types.AcquireSemaphoreRequest
	// Clock used to alter time in tests
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks and sets default parameters
func (l *SemaphoreLockConfig) CheckAndSetDefaults() error {
	if l.Clock == nil {
		l.Clock = clockwork.NewRealClock()
	}

	if l.Service == nil {
		return trace.BadParameter("missing semaphore service")
	}
	if l.Expiry == 0 {
		l.Expiry = defaults.SessionControlTimeout
	}
	if l.Expiry < time.Millisecond {
		return trace.BadParameter("sub-millisecond lease expiry is not supported: %v", l.Expiry)
	}
	if l.TickRate == 0 {
		l.TickRate = l.Expiry / 2
	}
	if l.TickRate >= l.Expiry {
		return trace.BadParameter("tick-rate must be less than expiry")
	}
	if l.Params.Expires.IsZero() {
		l.Params.Expires = l.Clock.Now().UTC().Add(l.Expiry)
	}
	if err := l.Params.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SemaphoreLock provides a convenient interface for managing
// semaphore lease keepalive operations.
// SemaphoreLock implements the [context.Context] interface
// and can be used to propagate cancellation when the parent
// context is canceled or when the lease expires.
//
//		lease,err := AcquireSemaphoreLock(ctx, cfg)
//		if err != nil {
//			... handle error ...
//		}
//		defer func(){
//			lease.Stop()
//			err := lease.Wait()
//			if err != nil {
//				... handle error ...
//			}
//		}()
//
//	 newCtx,cancel := context.WithCancel(ctx)
//	 defer cancel()
//	 ... do work with newCtx ...
type SemaphoreLock struct {
	// ctx is the parent context for the lease keepalive operation.
	// it's used to propagate deadline cancellations from the parent
	// context and to carry values for the context interface.
	ctx       context.Context
	cancelCtx context.CancelFunc
	cfg       SemaphoreLockConfig
	lease0    types.SemaphoreLease
	retry     retryutils.Retry
	ticker    clockwork.Ticker
	closeOnce sync.Once
	renewalC  chan struct{}
	cond      *sync.Cond
	err       error
	fin       bool
}

// finish registers the final result of the background
// goroutine.  must be called even if err is nil in
// order to wake any goroutines waiting on the error
// and mark the lock as finished.
func (l *SemaphoreLock) finish(err error) {
	l.cond.L.Lock()
	defer l.cond.L.Unlock()
	l.err = err
	l.fin = true
	l.cond.Broadcast()
}

// Done signals that lease keepalive operations
// have stopped.
// If the parent context is canceled, the lease
// will be released and done will be closed.
func (l *SemaphoreLock) Done() <-chan struct{} {
	return l.ctx.Done()
}

// Deadline returns the deadline of the parent context if it exists.
func (l *SemaphoreLock) Deadline() (time.Time, bool) {
	return l.ctx.Deadline()
}

// Value returns the value associated with the key in the parent context.
func (l *SemaphoreLock) Value(key interface{}) interface{} {
	return l.ctx.Value(key)
}

// Error returns the final error value.
func (l *SemaphoreLock) Err() error {
	l.cond.L.Lock()
	defer l.cond.L.Unlock()
	return l.err
}

// Wait blocks until the final result is available.  Note that
// this method may block longer than desired since cancellation of
// the parent context triggers the *start* of the release operation.
func (l *SemaphoreLock) Wait() error {
	l.cond.L.Lock()
	defer l.cond.L.Unlock()
	for !l.fin {
		l.cond.Wait()
	}
	return l.err
}

// Stop stops associated lease keepalive.
func (l *SemaphoreLock) Stop() {
	l.closeOnce.Do(func() {
		l.ticker.Stop()
		l.cancelCtx()
	})
}

// Renewed notifies on next successful lease keepalive.
// Used in tests to block until next renewal.
func (l *SemaphoreLock) Renewed() <-chan struct{} {
	return l.renewalC
}

func (l *SemaphoreLock) keepAlive(ctx context.Context) {
	var nodrop bool
	var err error
	lease := l.lease0
	defer func() {
		l.cancelCtx()
		l.Stop()
		defer l.finish(err)
		if nodrop {
			// non-standard exit conditions; don't bother handling
			// cancellation/expiry.
			return
		}
		if lease.Expires.After(l.cfg.Clock.Now().UTC()) {
			// parent context is closed. create orphan context with generous
			// timeout for lease cancellation scope.  this will not block any
			// caller that is not explicitly waiting on the final error value.
			cancelContext, cancel := context.WithTimeout(context.Background(), l.cfg.Expiry/4)
			defer cancel()
			err = l.cfg.Service.CancelSemaphoreLease(cancelContext, lease)
			if err != nil {
				log.Warnf("Failed to cancel semaphore lease %s/%s: %v", lease.SemaphoreKind, lease.SemaphoreName, err)
			}
		} else {
			log.Errorf("Semaphore lease expired: %s/%s", lease.SemaphoreKind, lease.SemaphoreName)
		}
	}()
Outer:
	for {
		select {
		case tick := <-l.ticker.Chan():
			leaseContext, leaseCancel := context.WithDeadline(ctx, lease.Expires)
			nextLease := lease
			nextLease.Expires = tick.Add(l.cfg.Expiry)
			for {
				err = l.cfg.Service.KeepAliveSemaphoreLease(leaseContext, nextLease)
				if trace.IsNotFound(err) {
					leaseCancel()
					// semaphore and/or lease no longer exist; best to log the error
					// and exit immediately.
					log.Warnf("Halting keepalive on semaphore %s/%s early: %v", lease.SemaphoreKind, lease.SemaphoreName, err)
					nodrop = true
					return
				}
				if err == nil {
					leaseCancel()
					lease = nextLease
					l.retry.Reset()
					select {
					case l.renewalC <- struct{}{}:
					default:
					}
					continue Outer
				}
				log.Debugf("Failed to renew semaphore lease %s/%s: %v", lease.SemaphoreKind, lease.SemaphoreName, err)
				l.retry.Inc()
				select {
				case <-l.retry.After():
				case tick = <-l.ticker.Chan():
					// check to make sure that we still have some time on the lease. the default tick rate would have
					// us waking _as_ the lease expires here, but if we're working with a higher tick rate, its worth
					// retrying again.
					if !lease.Expires.After(tick) {
						leaseCancel()
						return
					}
				case <-leaseContext.Done():
					leaseCancel() // demanded by linter
					return
				case <-l.Done():
					leaseCancel()
					return
				}
			}
		case <-ctx.Done():
			return
		case <-l.Done():
			return
		}
	}
}

// AcquireSemaphoreWithRetryConfig contains parameters for trying to acquire a
// semaphore with a retry.
type AcquireSemaphoreWithRetryConfig struct {
	Service types.Semaphores
	Request types.AcquireSemaphoreRequest
	Retry   retryutils.LinearConfig
}

// AcquireSemaphoreWithRetry tries to acquire the semaphore according to the
// retry schedule until it succeeds or context expires.
func AcquireSemaphoreWithRetry(ctx context.Context, req AcquireSemaphoreWithRetryConfig) (*types.SemaphoreLease, error) {
	retry, err := retryutils.NewLinear(req.Retry)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var lease *types.SemaphoreLease
	err = retry.For(ctx, func() (err error) {
		lease, err = req.Service.AcquireSemaphore(ctx, req.Request)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return lease, nil
}

// AcquireSemaphoreLock attempts to acquire and hold a semaphore lease.  If successfully acquired,
// background keepalive processes are started and an associated lock handle is returned. Canceling
// the supplied context releases the semaphore.
func AcquireSemaphoreLock(ctx context.Context, cfg SemaphoreLockConfig) (*SemaphoreLock, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// set up retry with a ratio which will result in 3-4 retries before the lease expires
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Max:    cfg.Expiry / 4,
		Step:   cfg.Expiry / 16,
		Jitter: retryutils.NewJitter(),
		Clock:  cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := cfg.Service.AcquireSemaphore(ctx, cfg.Params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	lock := &SemaphoreLock{
		ctx:       ctx,
		cancelCtx: cancel,
		cfg:       cfg,
		lease0:    *lease,
		retry:     retry,
		ticker:    cfg.Clock.NewTicker(cfg.TickRate),
		renewalC:  make(chan struct{}),
		cond:      sync.NewCond(&sync.Mutex{}),
	}
	go lock.keepAlive(ctx)
	return lock, nil
}

// SemaphoreLockConfigWithRetry contains parameters for acquiring a semaphore lock
// until it succeeds or context expires.
type SemaphoreLockConfigWithRetry struct {
	SemaphoreLockConfig
	// Retry is the retry configuration.
	Retry retryutils.LinearConfig
}

// AcquireSemaphoreLockWithRetry attempts to acquire and hold a semaphore lease. If successfully acquired,
// background keepalive processes are started and an associated lock handle is returned.
// If the lease cannot be acquired, the operation is retried according to the retry schedule until
// it succeeds or the context expires.  Canceling the supplied context releases the semaphore.
func AcquireSemaphoreLockWithRetry(ctx context.Context, cfg SemaphoreLockConfigWithRetry) (*SemaphoreLock, error) {
	retry, err := retryutils.NewLinear(cfg.Retry)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var lease *SemaphoreLock
	err = retry.For(ctx, func() (err error) {
		lease, err = AcquireSemaphoreLock(ctx, cfg.SemaphoreLockConfig)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return lease, nil
}

// UnmarshalSemaphore unmarshals the Semaphore resource from JSON.
func UnmarshalSemaphore(bytes []byte, opts ...MarshalOption) (types.Semaphore, error) {
	var semaphore types.SemaphoreV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(bytes, &semaphore); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	err = semaphore.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		semaphore.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		semaphore.SetExpiry(cfg.Expires)
	}
	return &semaphore, nil
}

// MarshalSemaphore marshals the Semaphore resource to JSON.
func MarshalSemaphore(semaphore types.Semaphore, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch semaphore := semaphore.(type) {
	case *types.SemaphoreV3:
		if err := semaphore.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, semaphore))
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", semaphore)
	}
}
