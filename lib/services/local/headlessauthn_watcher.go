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
	"errors"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// maxSubscribers is the maximum number of concurrent subscribers that a headless authentication watcher
// will accept. This limit is introduced because the headless login flow creates subscribers from an
// unauthenticated endpoint, which could be exploited in a ddos attack without the limit in place.
//
// 1024 was chosen as a reasonable limit, as under normal conditions, a single Teleport Cluster
// would never have over 1000 concurrent headless logins, each of which has a maximum lifetime
// of 30-60 seconds. If this limit is exceeded in a reasonable scenario, this limit should be
// made configurable in the server configuration file.
const maxSubscribers = 1024

var ErrHeadlessAuthenticationWatcherClosed = errors.New("headless authentication watcher closed")

// HeadlessAuthenticationWatcherConfig contains configuration options for a HeadlessAuthenticationWatcher.
type HeadlessAuthenticationWatcherConfig struct {
	// Backend is the storage backend used to create watchers.
	Backend backend.Backend
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

// HeadlessAuthenticationWatcher is a light weight backend watcher for the headless authentication resource.
type HeadlessAuthenticationWatcher struct {
	HeadlessAuthenticationWatcherConfig
	identityService *IdentityService
	retry           retryutils.Retry
	mux             sync.Mutex
	subscribers     [maxSubscribers]*headlessAuthenticationSubscriber
	closed          chan struct{}
	running         chan struct{}
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

	h := &HeadlessAuthenticationWatcher{
		HeadlessAuthenticationWatcherConfig: cfg,
		identityService:                     NewIdentityService(cfg.Backend),
		retry:                               retry,
		closed:                              make(chan struct{}),
		running:                             make(chan struct{}),
	}

	go h.runWatchLoop(ctx)

	return h, nil
}

// WaitInit waits for the watch loop to initialize.
func (h *HeadlessAuthenticationWatcher) WaitInit(ctx context.Context) error {
	select {
	case <-h.running:
	case <-ctx.Done():
	}
	return trace.Wrap(ctx.Err())
}

// Done returns a channel that's closed when the watcher is closed.
func (h *HeadlessAuthenticationWatcher) Done() <-chan struct{} {
	return h.closed
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
			h.Log.Warningf("Restarting watch on error after waiting %v. Error: %v.", t.Sub(startedWaiting), err)
			h.retry.Inc()
		case <-ctx.Done():
			h.Log.WithError(ctx.Err()).Debugf("Context closed with err. Returning from watch loop.")
			return
		case <-h.closed:
			h.Log.Debug("Watcher closed. Returning from watch loop.")
			return
		}
	}
}

func (h *HeadlessAuthenticationWatcher) watch(ctx context.Context) error {
	watcher, err := h.newWatcher(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	// Notify any subscribers initiated before the new watcher initialized.
	headlessAuthns, err := h.identityService.GetHeadlessAuthentications(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	h.notify(headlessAuthns...)

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
			return errors.New("watcher closed")
		case <-ctx.Done():
			return ctx.Err()
		case h.running <- struct{}{}:
		}
	}
}

func (h *HeadlessAuthenticationWatcher) newWatcher(ctx context.Context) (backend.Watcher, error) {
	watcher, err := h.identityService.NewWatcher(ctx, backend.Watch{
		Name:            types.KindHeadlessAuthentication,
		MetricComponent: types.KindHeadlessAuthentication,
		Prefixes:        [][]byte{headlessAuthenticationKey("")},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	select {
	case <-watcher.Done():
		return nil, errors.New("watcher closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return nil, trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	h.retry.Reset()
	return watcher, nil
}

func (h *HeadlessAuthenticationWatcher) notify(headlessAuthns ...*types.HeadlessAuthentication) {
	h.mux.Lock()
	defer h.mux.Unlock()

	for _, ha := range headlessAuthns {
		for _, s := range h.subscribers {
			if s != nil && (s.name == ha.Metadata.Name || s.name == ha.User) {
				select {
				case s.updates <- apiutils.CloneProtoMsg(ha):
				default:
					select {
					case s.stale <- struct{}{}:
					default:
						// subscriber is already stale, skip.
					}
				}
			}
		}
	}
}

// HeadlessAuthenticationSubscriber is a subscriber for headless authentication updates
// matching the subscriber name by ID or username.
type HeadlessAuthenticationSubscriber interface {
	Name() string
	// Updates is a channel used by the watcher to send headless authentication updates.
	// After receiving an update, the caller should check the stale channel to ensure it
	// is not a stale update.
	Updates() <-chan *types.HeadlessAuthentication
	// Stale is a channel used by the watcher to notify the subscriber that one or more
	// updates have been missed, due to slow receives on the Updates channel.
	Stale() <-chan struct{}
	// Close closes the subscriber and its channels. This frees up resources for the watcher
	// and should always be called on completion.
	Close()
}

// Subscribe creates a subscriber for headless authentications matching the given name,
// which can be either a specific headless authentication ID or a username.
func (h *HeadlessAuthenticationWatcher) Subscribe(ctx context.Context, name string) (HeadlessAuthenticationSubscriber, error) {
	i, err := h.assignSubscriber(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	subscriber := h.subscribers[i]

	go func() {
		select {
		case <-ctx.Done():
		case <-subscriber.closed:
		}

		// reclaim the subscriber and close remaining open channels.
		h.unassignSubscriber(i)
		close(subscriber.updates)
		close(subscriber.stale)
	}()

	return subscriber, nil
}

func (h *HeadlessAuthenticationWatcher) assignSubscriber(name string) (int, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	select {
	case <-h.closed:
		return 0, ErrHeadlessAuthenticationWatcherClosed
	default:
	}

	for i := range h.subscribers {
		if h.subscribers[i] == nil {
			h.subscribers[i] = &headlessAuthenticationSubscriber{
				name: name,
				// small buffer for updates to avoid unnecessary stale checks.
				updates: make(chan *types.HeadlessAuthentication, 1),
				// buffer required to mark as stale.
				stale:  make(chan struct{}, 1),
				closed: make(chan struct{}),
			}
			return i, nil
		}
	}

	return 0, trace.LimitExceeded("too many in-flight headless login requests")
}

func (h *HeadlessAuthenticationWatcher) unassignSubscriber(i int) {
	h.mux.Lock()
	defer h.mux.Unlock()
	h.subscribers[i] = nil
}

// headlessAuthenticationSubscriber is a subscriber for headless authentication updates
// matching the subscriber name by ID or username.
type headlessAuthenticationSubscriber struct {
	// name is a headless authentication ID or username to subscribe to.
	name string
	// updates is a channel used by the watcher to send resource updates.
	updates chan *types.HeadlessAuthentication
	// stale is a channel used to determine if the subscriber is stale and
	// needs to check the backend for missed data.
	stale chan struct{}
	// closed is a channel used to determine if the subscriber is closed.
	closed chan struct{}
}

func (s *headlessAuthenticationSubscriber) Name() string {
	return s.name
}

func (s *headlessAuthenticationSubscriber) Updates() <-chan *types.HeadlessAuthentication {
	return s.updates
}

func (s *headlessAuthenticationSubscriber) Stale() <-chan struct{} {
	return s.stale
}

func (s *headlessAuthenticationSubscriber) Close() {
	close(s.closed)
}

// WaitForUpdate waits until the headless authentication with the given name is updated in the
// backend to meet the given condition or returns early if the condition results in an
// error or if the watcher or given context is closed.
func (h *HeadlessAuthenticationWatcher) WaitForUpdate(ctx context.Context, subscriber HeadlessAuthenticationSubscriber, cond func(*types.HeadlessAuthentication) (bool, error)) (*types.HeadlessAuthentication, error) {
	checkBackend := func() (*types.HeadlessAuthentication, error) {
		has, err := h.identityService.GetHeadlessAuthentications(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Return the first entry that matches the subscriber and passes the condition.
		for _, ha := range has {
			if ha.GetName() == subscriber.Name() || ha.User == subscriber.Name() {
				if ok, err := cond(ha); err != nil {
					return nil, trace.Wrap(err)
				} else if ok {
					return ha, nil
				}
			}
		}
		return nil, nil
	}

	// First check the backend for an existing entry.
	ha, err := checkBackend()
	if err != nil {
		return nil, trace.Wrap(err)
	} else if ha != nil {
		return ha, nil
	}

	for {
		select {
		case ha := <-subscriber.Updates():
			select {
			case <-subscriber.Stale():
				// Stale update, check the backend.
				if ha, err := checkBackend(); err != nil {
					return nil, trace.Wrap(err)
				} else if ha != nil {
					return ha, nil
				}
			default:
				if ok, err := cond(ha); err != nil {
					return nil, trace.Wrap(err)
				} else if ok {
					return ha, nil
				}
			}
		case <-subscriber.Stale():
			// Drain stale update channel.
			select {
			case <-subscriber.Updates():
			default:
			}

			if ha, err := checkBackend(); err != nil {
				return nil, trace.Wrap(err)
			} else if ha != nil {
				return ha, nil
			}
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-h.Done():
			return nil, ErrHeadlessAuthenticationWatcherClosed
		}
	}
}
