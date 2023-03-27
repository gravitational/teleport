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
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
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

type HeadlessAuthenticationWatcherConfig struct {
	// Backend is the storage backend used to create watchers.
	Backend backend.Backend
	// WatcherService is a service used to create new watchers.
	// If nil, Backend will be used as the watcher service.
	WatcherService interface {
		NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error)
	}
	// Log is a logger.
	Log logrus.FieldLogger
	// Clock is used to control time.
	Clock clockwork.Clock
	// MaxRetryPeriod is the maximum retry period on failed watchers.
	MaxRetryPeriod time.Duration
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *HeadlessAuthenticationWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing parameter Backend")
	}
	if cfg.WatcherService == nil {
		cfg.WatcherService = cfg.Backend
	}
	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
		cfg.Log.WithField("resource-kind", types.KindHeadlessAuthentication)
	}
	if cfg.MaxRetryPeriod == 0 {
		// On watcher failure, we eagerly retry in order to avoid login delays.
		cfg.MaxRetryPeriod = defaults.HighResPollingPeriod
	}
	if cfg.Clock == nil {
		cfg.Clock = cfg.Backend.Clock()
	}
	return nil
}

// HeadlessAuthenticationWatcher is a custom backend watcher for the headless authentication resource.
type HeadlessAuthenticationWatcher struct {
	HeadlessAuthenticationWatcherConfig
	identityService *IdentityService
	retry           retryutils.Retry
	mux             sync.Mutex
	waiters         [maxWaiters]headlessAuthenticationWaiter
	closed          chan struct{}
}

// NewHeadlessAuthenticationWatcher creates a new headless authentication resource watcher.
// The watcher will close once the given ctx is closed.
func NewHeadlessAuthenticationWatcher(ctx context.Context, cfg HeadlessAuthenticationWatcherConfig) (*HeadlessAuthenticationWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  utils.FullJitter(cfg.MaxRetryPeriod / 10),
		Step:   cfg.MaxRetryPeriod / 5,
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.NewHalfJitter(),
		Clock:  cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watcher := &HeadlessAuthenticationWatcher{
		HeadlessAuthenticationWatcherConfig: cfg,
		identityService:                     NewIdentityService(cfg.Backend),
		retry:                               retry,
		closed:                              make(chan struct{}),
	}

	go watcher.runWatchLoop(ctx)

	return watcher, nil
}

func (h *HeadlessAuthenticationWatcher) close() {
	h.mux.Lock()
	defer h.mux.Unlock()
	close(h.closed)
}

func (h *HeadlessAuthenticationWatcher) runWatchLoop(ctx context.Context) {
	defer h.close()
	for {
		err := h.watch(ctx)

		startedWaiting := h.Clock.Now()
		select {
		case t := <-h.retry.After():
			h.Log.Debugf("Attempting to restart watch after waiting %v.", t.Sub(startedWaiting))
			h.retry.Inc()
		case <-ctx.Done():
			h.Log.Debug("Context closed, returning from watch loop.")
			return
		case <-h.closed:
			h.Log.Debug("Watcher closed, returning from watch loop.")
			return
		}
		if err != nil {
			h.Log.Warningf("Restart watch on error: %v.", err)
		}
	}
}

func (h *HeadlessAuthenticationWatcher) watch(ctx context.Context) error {
	watcher, err := h.WatcherService.NewWatcher(ctx, backend.Watch{
		Name:            types.KindHeadlessAuthentication,
		MetricComponent: types.KindHeadlessAuthentication,
		Prefixes:        [][]byte{headlessAuthenticationKey("")},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	select {
	case <-watcher.Done():
		return fmt.Errorf("watcher closed")
	case <-ctx.Done():
		return ctx.Err()
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	headlessAuthns, err := h.identityService.GetHeadlessAuthentications(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Notify any waiters initiated before the new watcher initialized.
	h.notify(headlessAuthns...)
	h.retry.Reset()

	for {
		select {
		case event := <-watcher.Events():
			switch event.Type {
			case types.OpPut:
				headlessAuthn, err := unmarshalHeadlessAuthenticationFromItem(&event.Item)
				if err != nil {
					h.Log.WithError(err).Debug("failed to unmarshal headless authentication from put event")
				} else {
					h.notify(headlessAuthn)
				}
			}
		case <-watcher.Done():
			return fmt.Errorf("watcher closed")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (h *HeadlessAuthenticationWatcher) notify(headlessAuthns ...*types.HeadlessAuthentication) {
	h.mux.Lock()
	defer h.mux.Unlock()
	for _, headlessAuthn := range headlessAuthns {
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
}

// Wait watches for the headless authentication with the given id to be added/updated
// in the backend, and waits for the given condition to be met, to result in an error,
// or for the given context to close.
func (h *HeadlessAuthenticationWatcher) Wait(ctx context.Context, name string, cond func(*types.HeadlessAuthentication) (bool, error)) (*types.HeadlessAuthentication, error) {
	checkBackend := func() (*types.HeadlessAuthentication, bool, error) {
		headlessAuthn, err := h.identityService.GetHeadlessAuthentication(ctx, name)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		ok, err := cond(headlessAuthn)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}

		return headlessAuthn, ok, nil
	}

	// Before the waiter is allocated, check if there is an existing entry in the backend.
	headlessAuthn, ok, err := checkBackend()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	} else if ok {
		return headlessAuthn, nil
	}

	waiter, err := h.assignWaiter(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer h.unassignWaiter(waiter)

	for {
		select {
		case <-waiter.stale:
			// If the waiter is a slow consumer it may be marked as stale, in which
			// case it should check the backend for the latest resource version.
			h.unmarkStale(waiter)

			headlessAuthn, ok, err := checkBackend()
			if err != nil {
				return nil, trace.Wrap(err)
			} else if ok {
				return headlessAuthn, nil
			}
		case headlessAuthn := <-waiter.ch:
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
	h.mux.Lock()
	defer h.mux.Unlock()

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
	h.mux.Lock()
	defer h.mux.Unlock()

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
	h.mux.Lock()
	defer h.mux.Unlock()
	waiter.stale = make(chan struct{})
}
