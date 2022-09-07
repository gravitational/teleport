/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

func lockKey(parts ...string) []byte {
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

// AcquireLock grabs a lock that will be released automatically in TTL
func AcquireLock(ctx context.Context, backend Backend, lockName string, ttl time.Duration) (Lock, error) {
	if lockName == "" {
		return Lock{}, trace.BadParameter("missing parameter lock name")
	}
	key := lockKey(lockName)
	id, err := randomID()
	if err != nil {
		return Lock{}, trace.Wrap(err)
	}
	for {
		// Get will clear TTL on a lock
		backend.Get(ctx, key)

		// CreateVal is atomic:
		_, err = backend.Create(ctx, Item{Key: key, Value: id, Expires: backend.Clock().Now().UTC().Add(ttl)})
		if err == nil {
			break // success
		}
		if trace.IsAlreadyExists(err) { // locked? wait and repeat:
			select {
			case <-backend.Clock().After(250 * time.Millisecond):
				// OK, go around and try again
				continue

			case <-ctx.Done():
				// Context has been canceled externally, time to go
				return Lock{}, trace.Wrap(ctx.Err())
			}
		}
		return Lock{}, trace.ConvertSystemError(err)
	}
	return Lock{key: key, id: id, ttl: ttl}, nil
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

// RunWhileLocked allows you to run a function while a lock is held.
func RunWhileLocked(ctx context.Context, backend Backend, lockName string, ttl time.Duration, fn func(context.Context) error) error {
	lock, err := AcquireLock(ctx, backend, lockName, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	subContext, cancelFunction := context.WithCancel(ctx)

	stopRefresh := make(chan struct{})
	go func() {
		refreshAfter := ttl / 2
		for {
			select {
			case <-time.After(refreshAfter):
				if err := lock.resetTTL(ctx, backend); err != nil {
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

	if err := lock.Release(ctx, backend); err != nil {
		return trace.NewAggregate(fnErr, err)
	}

	return fnErr
}
