/*
Copyright 2023 Gravitational, Inc.

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

package local

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// maxWaiters is the maximum number of concurrent waiters that a headless authentication watcher
// will accept. This limit is introduced because the headless login flow creates waiters from an
// unauthenticated endpoint, which could be exploited in a ddos attack without the limit in place.
//
// 1024 was chosen as a reasonable limit, as under normal conditions, a single Teleport Cluster
// would never have over 1000 concurrent headless logins, each of which has a maximum lifetime
// of 30-60 seconds. If this limit is exceeded in a reasonable scenario, this limit should be
// made configurable in the server configuration file.
const maxWaiters = 1024

var watcherClosedErr = trace.Errorf("headless authentication watcher closed")

// HeadlessAuthenticationWatcher is a custom backend watcher for the headless authentication resource.
type HeadlessAuthenticationWatcher struct {
	log         logrus.FieldLogger
	b           backend.Backend
	watchersMux sync.Mutex
	waiters     [maxWaiters]headlessAuthenticationWaiter
	closed      chan struct{}
}

// NewHeadlessAuthenticationWatcher creates a new headless authentication resource watcher.
// The watcher will close once the given ctx is closed.
func NewHeadlessAuthenticationWatcher(ctx context.Context, b backend.Backend) (*HeadlessAuthenticationWatcher, error) {
	if b == nil {
		return nil, trace.BadParameter("missing required field backend")
	}
	watcher := &HeadlessAuthenticationWatcher{
		log:    logrus.StandardLogger(),
		b:      b,
		closed: make(chan struct{}),
	}

	if err := watcher.start(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return watcher, nil
}

func (h *HeadlessAuthenticationWatcher) start(ctx context.Context) error {
	w, err := h.b.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{headlessAuthenticationKey("")}})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		defer w.Close()
		for {
			select {
			case event := <-w.Events():
				switch event.Type {
				case types.OpPut:
					headlessAuthn, err := unmarshalHeadlessAuthenticationFromItem(&event.Item)
					if err != nil {
						h.log.WithError(err).Debug("failed to unmarshal headless authentication from put event")
					} else {
						h.notify(headlessAuthn)
					}
				}
			case <-ctx.Done():
				h.close()
				return
			}
		}
	}()

	return nil
}

func (h *HeadlessAuthenticationWatcher) close() {
	h.watchersMux.Lock()
	defer h.watchersMux.Unlock()
	close(h.closed)
}

func (h *HeadlessAuthenticationWatcher) notify(headlessAuthn *types.HeadlessAuthentication) {
	h.watchersMux.Lock()
	defer h.watchersMux.Unlock()
	for i := range h.waiters {
		if h.waiters[i].name == headlessAuthn.Metadata.Name {
			select {
			case h.waiters[i].ch <- headlessAuthn:
			default:
				h.markStaleUnderLock(&h.waiters[i])
			}
		}
	}
}

// CheckWaiter checks if there is an active waiter matching the given
// headless authentication ID. Used in tests.
func (h *HeadlessAuthenticationWatcher) CheckWaiter(name string) bool {
	h.watchersMux.Lock()
	defer h.watchersMux.Unlock()
	for i := range h.waiters {
		if h.waiters[i].name == name {
			return true
		}
	}
	return false
}

// Wait watches for the headless authentication with the given id to be added/updated
// in the backend, and waits for the given condition to be met, to result in an error,
// or for the given context to close.
func (h *HeadlessAuthenticationWatcher) Wait(ctx context.Context, name string, cond func(*types.HeadlessAuthentication) (bool, error)) (*types.HeadlessAuthentication, error) {
	waiter, err := h.assignWaiter(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer h.unassignWaiter(waiter)

	checkBackend := func() (*types.HeadlessAuthentication, bool, error) {
		currentItem, err := h.b.Get(ctx, headlessAuthenticationKey(name))
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		headlessAuthn, err := unmarshalHeadlessAuthenticationFromItem(currentItem)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		ok, err := cond(headlessAuthn)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		return headlessAuthn, ok, nil
	}

	// With the waiter allocated, check if there is an existing entry in the backend.
	headlessAuthn, ok, err := checkBackend()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	} else if ok {
		return headlessAuthn, nil
	}

	for {
		select {
		case <-waiter.stale:
			// If the waiter is a slow consumer it may be marked as stale, in which
			// case it should check the backend for the latest resource version.
			h.unmarkStale(waiter)

			if headlessAuthn, ok, err := checkBackend(); err != nil {
				return nil, trace.Wrap(err)
			} else if ok {
				return headlessAuthn, nil
			}
		case headlessAuthn := <-waiter.ch:
			select {
			case <-waiter.stale:
				// prioritize stale check.
				continue
			default:
			}
			if ok, err := cond(headlessAuthn); err != nil {
				return nil, trace.Wrap(err)
			} else if ok {
				return headlessAuthn, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-h.closed:
			return nil, watcherClosedErr
		}
	}
}

func (h *HeadlessAuthenticationWatcher) assignWaiter(ctx context.Context, name string) (*headlessAuthenticationWaiter, error) {
	h.watchersMux.Lock()
	defer h.watchersMux.Unlock()

	select {
	case <-h.closed:
		return nil, watcherClosedErr
	default:
	}

	for i := range h.waiters {
		if h.waiters[i].ch != nil {
			continue
		}
		h.waiters[i].ch = make(chan *types.HeadlessAuthentication)
		h.waiters[i].name = name
		h.waiters[i].stale = make(chan struct{})
		return &h.waiters[i], nil
	}

	return nil, trace.LimitExceeded("too many in-flight headless login requests")
}

func (h *HeadlessAuthenticationWatcher) unassignWaiter(waiter *headlessAuthenticationWaiter) {
	h.watchersMux.Lock()
	defer h.watchersMux.Unlock()

	// close channels.
	close(waiter.ch)
	h.markStaleUnderLock(waiter)

	waiter.ch = nil
	waiter.name = ""
	waiter.stale = nil
}

// headlessAuthenticationWaiter is a waiter for a specific headless authentication.
type headlessAuthenticationWaiter struct {
	// name is the name of the headless authentication resource being waited on.
	name string
	// ch is a channel used by the watcher to send resource updates.
	ch chan *types.HeadlessAuthentication
	// stale is a channel used to determine if the waiter is stale and
	// needs to check the backend for missed data. The watcher will close
	// this channel when it misses an update.
	stale chan struct{}
}

// markStaleUnderLock marks a waiter as stale so it will update itself once available.
// This should be called when a waiter misses an update due to slow consumption on its channel.
//
// must be called by HeadlessAuthenticationWatcher under watcherMux
func (h *HeadlessAuthenticationWatcher) markStaleUnderLock(waiter *headlessAuthenticationWaiter) {
	select {
	case <-waiter.stale:
	default:
		close(waiter.stale)
	}
}

// unmarkStale marks a waiter as not stale. This should be called when the waiter performs a stale check.
func (h *HeadlessAuthenticationWatcher) unmarkStale(waiter *headlessAuthenticationWaiter) {
	h.watchersMux.Lock()
	defer h.watchersMux.Unlock()
	waiter.stale = make(chan struct{})
}
