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

package backend

import (
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	flagsPrefix = ".flags"
	locksPrefix = ".locks"
)

func FlagKey(parts ...string) []byte {
	return internalKey(flagsPrefix, parts...)
}

func LockKey(parts ...string) []byte {
	return internalKey(locksPrefix, parts...)
}

type Lock struct {
	key []byte
	id  []byte
	ttl time.Duration
}

func randomID() ([]byte, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bytes := [16]byte(uuid)
	return bytes[:], nil
}

type LockConfiguration struct {
	Backend  Backend
	LockName string
	// TTL defines when lock will be released automatically
	TTL time.Duration
	// RetryInterval defines interval which is used to retry locking after
	// initial lock failed due to someone else holding lock.
	RetryInterval time.Duration
}

func (l *LockConfiguration) CheckAndSetDefaults() error {
	if l.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if l.LockName == "" {
		return trace.BadParameter("missing LockName")
	}
	if l.TTL == 0 {
		return trace.BadParameter("missing TTL")
	}
	if l.RetryInterval == 0 {
		l.RetryInterval = 250 * time.Millisecond
	}
	return nil
}

// AcquireLock grabs a lock that will be released automatically in TTL
func AcquireLock(ctx context.Context, cfg LockConfiguration) (Lock, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return Lock{}, trace.Wrap(err)
	}
	key := LockKey(cfg.LockName)
	id, err := randomID()
	if err != nil {
		return Lock{}, trace.Wrap(err)
	}
	for {
		// Get will clear TTL on a lock
		cfg.Backend.Get(ctx, key)

		// CreateVal is atomic:
		_, err = cfg.Backend.Create(ctx, Item{Key: key, Value: id, Expires: cfg.Backend.Clock().Now().UTC().Add(cfg.TTL)})
		if err == nil {
			break // success
		}
		if trace.IsAlreadyExists(err) { // locked? wait and repeat:
			select {
			case <-cfg.Backend.Clock().After(cfg.RetryInterval):
				// OK, go around and try again
				continue

			case <-ctx.Done():
				// Context has been canceled externally, time to go
				return Lock{}, trace.Wrap(ctx.Err())
			}
		}
		return Lock{}, trace.ConvertSystemError(err)
	}
	return Lock{key: key, id: id, ttl: cfg.TTL}, nil
}

// Release forces lock release
func (l *Lock) Release(ctx context.Context, backend Backend) error {
	prev, err := backend.Get(ctx, l.key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.CompareFailed("cannot release lock %s (expired)", l.id)
		}
		return trace.Wrap(err)
	}

	if !bytes.Equal(prev.Value, l.id) {
		return trace.CompareFailed("cannot release lock %s (ownership changed)", l.id)
	}

	if err := backend.Delete(ctx, l.key); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// resetTTL resets the TTL on a given lock.
func (l *Lock) resetTTL(ctx context.Context, backend Backend) error {
	prev, err := backend.Get(ctx, l.key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.CompareFailed("cannot refresh lock %s (expired)", l.id)
		}
		return trace.Wrap(err)
	}

	if !bytes.Equal(prev.Value, l.id) {
		return trace.CompareFailed("cannot refresh lock %s (ownership changed)", l.id)
	}

	next := *prev
	next.Expires = backend.Clock().Now().UTC().Add(l.ttl)

	_, err = backend.CompareAndSwap(ctx, *prev, next)
	if err != nil {
		return trace.WrapWithMessage(err, "failed to fresh lock %s (cas failed)", l.id)
	}

	return nil
}

// RunWhileLockedConfig is configuration for RunWhileLocked function.
type RunWhileLockedConfig struct {
	// LockConfiguration is configuration for acquire lock.
	LockConfiguration

	// ReleaseCtxTimeout defines timeout used for calling lock.Release method (optional).
	ReleaseCtxTimeout time.Duration
	// RefreshLockInterval defines interval at which lock will be refreshed
	// if fn is still running (optional).
	RefreshLockInterval time.Duration
}

func (c *RunWhileLockedConfig) CheckAndSetDefaults() error {
	if err := c.LockConfiguration.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.ReleaseCtxTimeout <= 0 {
		c.ReleaseCtxTimeout = time.Second
	}
	if c.RefreshLockInterval <= 0 {
		c.RefreshLockInterval = c.LockConfiguration.TTL / 2
	}
	return nil
}

// RunWhileLocked allows you to run a function while a lock is held.
func RunWhileLocked(ctx context.Context, cfg RunWhileLockedConfig, fn func(context.Context) error) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	lock, err := AcquireLock(ctx, cfg.LockConfiguration)
	if err != nil {
		return trace.Wrap(err)
	}

	subContext, cancelFunction := context.WithCancel(ctx)
	defer cancelFunction()

	stopRefresh := make(chan struct{})
	go func() {
		refreshAfter := cfg.RefreshLockInterval
		for {
			select {
			case <-cfg.Backend.Clock().After(refreshAfter):
				if err := lock.resetTTL(ctx, cfg.Backend); err != nil {
					cancelFunction()
					log.Errorf("%v", err)
					return
				}
			case <-stopRefresh:
				return
			}
		}
	}()

	fnErr := fn(subContext)
	close(stopRefresh)

	// Release the lock with a separate context to allow the lock to be removed even
	// if the passed in context is canceled by the user, otherwise the lock will be
	// left around for the entire TTL even though the operation that required the
	// lock may have completed.
	releaseLockCtx, releaseLockCancel := context.WithTimeout(context.Background(), cfg.ReleaseCtxTimeout)
	defer releaseLockCancel()
	if err := lock.Release(releaseLockCtx, cfg.Backend); err != nil {
		return trace.NewAggregate(fnErr, err)
	}

	return fnErr
}
