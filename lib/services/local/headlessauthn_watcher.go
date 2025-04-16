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

package local

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
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
	// Logger is a logger.
	Logger *slog.Logger
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
	if cfg.Logger == nil {
		cfg.Logger = slog.With("resource_kind", types.KindHeadlessAuthentication)
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
	sync.Mutex
	subscribers [maxSubscribers]*headlessAuthenticationSubscriber
	closed      chan struct{}
	running     chan struct{}
}

// NewHeadlessAuthenticationWatcher creates a new headless authentication resource watcher.
// The watcher will close once the given ctx is closed.
func NewHeadlessAuthenticationWatcher(ctx context.Context, cfg HeadlessAuthenticationWatcherConfig) (*HeadlessAuthenticationWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.FullJitter(cfg.MaxRetryPeriod / 10),
		Step:   cfg.MaxRetryPeriod / 5,
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identityService, err := NewIdentityService(cfg.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &HeadlessAuthenticationWatcher{
		HeadlessAuthenticationWatcherConfig: cfg,
		identityService:                     identityService,
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
	h.Lock()
	defer h.Unlock()
	close(h.closed)

	for _, s := range h.subscribers {
		if s != nil {
			s.Close()
		}
	}
}

func (h *HeadlessAuthenticationWatcher) runWatchLoop(ctx context.Context) {
	defer h.close()
	for {
		err := h.watch(ctx)

		startedWaiting := h.Clock.Now()
		select {
		case t := <-h.retry.After():
			h.Logger.WarnContext(ctx, "Restarting watch on error",
				"backoff", t.Sub(startedWaiting),
				"error", err,
			)
			h.retry.Inc()
		case <-ctx.Done():
			return
		case <-h.closed:
			h.Logger.DebugContext(ctx, "Watcher closed, terminating watch loop")
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
					h.Logger.DebugContext(ctx, "failed to unmarshal headless authentication from put event", "error", err)
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
		Prefixes:        []backend.Key{backend.NewKey(headlessAuthenticationPrefix)},
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
	h.Lock()
	defer h.Unlock()

	for _, ha := range headlessAuthns {
		for _, s := range h.subscribers {
			if s != nil && s.name == ha.Metadata.Name && s.username == ha.User {
				s.update(ha, true)
			}
		}
	}
}

// HeadlessAuthenticationSubscriber is a subscriber for a specific headless authentication.
type HeadlessAuthenticationSubscriber interface {
	// Updates is a channel used by the watcher to send headless authentication updates.
	Updates() <-chan *types.HeadlessAuthentication
	// WaitForUpdate returns the first update which passes the given condition, or returns
	// early if the condition results in an error or if the subscriber or given context is closed.
	WaitForUpdate(ctx context.Context, cond func(*types.HeadlessAuthentication) (bool, error)) (*types.HeadlessAuthentication, error)
	// Done returns a channel that's closed when the subscriber is closed.
	Done() <-chan struct{}
	// Close closes the subscriber and its channels. This frees up resources for the watcher
	// and should always be called on completion.
	Close()
}

// Subscribe creates a subscriber for a specific headless authentication.
func (h *HeadlessAuthenticationWatcher) Subscribe(ctx context.Context, username, name string) (HeadlessAuthenticationSubscriber, error) {
	if name == "" {
		return nil, trace.BadParameter("name must be provided")
	}
	if username == "" {
		return nil, trace.BadParameter("username must be provided")
	}

	i, err := h.assignSubscriber(username, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	subscriber := h.subscribers[i]

	go func() {
		<-subscriber.Done()
		h.unassignSubscriber(i)
	}()

	// Check for an existing backend entry and send it as the first update.
	if ha, err := h.identityService.GetHeadlessAuthentication(ctx, username, name); err == nil {
		// If the subscriber receives an event before we finish checking the
		// current backend, we can just skip this update rather than overwriting
		// the more up to date event.
		subscriber.update(ha, false /* overwrite */)
	} else if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	return subscriber, nil
}

func (h *HeadlessAuthenticationWatcher) assignSubscriber(username, name string) (int, error) {
	h.Lock()
	defer h.Unlock()

	select {
	case <-h.closed:
		return 0, ErrHeadlessAuthenticationWatcherClosed
	default:
	}

	for i := range h.subscribers {
		if h.subscribers[i] == nil {
			h.subscribers[i] = &headlessAuthenticationSubscriber{
				name:     name,
				username: username,
				// small buffer for updates so we can replace stale updates.
				updates: make(chan *types.HeadlessAuthentication, 1),
				closed:  make(chan struct{}),
			}
			return i, nil
		}
	}

	return 0, trace.LimitExceeded("too many in-flight headless login requests")
}

func (h *HeadlessAuthenticationWatcher) unassignSubscriber(i int) {
	h.Lock()
	defer h.Unlock()
	h.subscribers[i] = nil
}

// headlessAuthenticationSubscriber is a subscriber for a specific headless authentication.
type headlessAuthenticationSubscriber struct {
	// name is a headless authentication name.
	name string
	// username is a teleport username.
	username string
	// updates is a channel used by the watcher to send resource updates. This channel
	// will either be empty or have the latest update in its buffer.
	updates   chan *types.HeadlessAuthentication
	updatesMu sync.Mutex
	// closed is a channel used to determine if the subscriber is closed.
	closed chan struct{}
}

// Updates is a channel used by the watcher to send headless authentication updates.
func (s *headlessAuthenticationSubscriber) Updates() <-chan *types.HeadlessAuthentication {
	return s.updates
}

// WaitForUpdate returns the first update which passes the given condition, or returns
// early if the condition results in an error or if the subscriber or given context is closed.
func (s *headlessAuthenticationSubscriber) WaitForUpdate(ctx context.Context, cond func(*types.HeadlessAuthentication) (bool, error)) (*types.HeadlessAuthentication, error) {
	for {
		select {
		case ha, ok := <-s.Updates():
			if !ok {
				return nil, ErrHeadlessAuthenticationWatcherClosed
			}
			if ok, err := cond(ha); err != nil {
				return nil, trace.Wrap(err)
			} else if ok {
				return ha, nil
			}
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-s.Done():
			return nil, ErrHeadlessAuthenticationWatcherClosed
		}
	}
}

// Done returns a channel that's closed when the subscriber is closed.
func (s *headlessAuthenticationSubscriber) Done() <-chan struct{} {
	return s.closed
}

// Close closes the subscriber and its channels. This frees up resources for the watcher
// and should always be called on completion.
func (s *headlessAuthenticationSubscriber) Close() {
	s.updatesMu.Lock()
	defer s.updatesMu.Unlock()

	select {
	case <-s.closed:
	default:
		close(s.closed)
		close(s.updates)
	}
}

func (s *headlessAuthenticationSubscriber) update(ha *types.HeadlessAuthentication, overwrite bool) {
	s.updatesMu.Lock()
	defer s.updatesMu.Unlock()

	select {
	case <-s.closed:
		// subscriber is closing, ignore updates.
		return
	default:
	}

	// Drain stale update if there is one.
	if overwrite {
		select {
		case _, ok := <-s.updates:
			if !ok {
				// updates channel is closed, subscriber is closing.
				return
			}
		default:
		}
	}

	select {
	case s.updates <- apiutils.CloneProtoMsg(ha):
	default:
	}
}
