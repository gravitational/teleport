/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package access

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache/assignments"
	"github.com/gravitational/teleport/lib/scopes/cache/roles"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// CacheConfig configures the scoped access cache.
type CacheConfig struct {
	Events            types.Events
	Reader            services.ScopedAccessReader
	MaxRetryPeriod    time.Duration
	TTLCacheRetention time.Duration
}

// CheckAndSetDefaults verifies required fields and sets default values as appropriate.
func (c *CacheConfig) CheckAndSetDefaults() error {
	if c.Events == nil {
		return trace.BadParameter("missing required parameter Events in scoped access cache config")
	}

	if c.Reader == nil {
		return trace.BadParameter("missing required parameter Reader in scoped access cache config")
	}

	if c.MaxRetryPeriod <= 0 {
		c.MaxRetryPeriod = defaults.MaxLongWatcherBackoff
	}

	if c.TTLCacheRetention <= 0 {
		c.TTLCacheRetention = time.Second * 3
	}

	return nil
}

// state holds the cache state elements.
type state struct {
	roles       *roles.RoleCache
	assignments *assignments.AssignmentCache
}

// Cache is an in-memory cache for scoped access resources. It provides similar features to the primary
// teleport cache, but is specifically tailored to support scope-based queries that are difficult to implement
// with the primary cache.
type Cache struct {
	cfg      CacheConfig
	rw       sync.RWMutex
	state    state
	ok       bool
	closed   bool
	cancel   context.CancelFunc
	ttlCache *utils.FnCache
	done     chan struct{}
}

// NewCache attempts to configure and start a new scoped access cache. The cache is immediately readable if returned,
// but performance may be suboptimal until watcher init has completed.
func NewCache(cfg CacheConfig) (*Cache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  retryutils.FullJitter(cfg.MaxRetryPeriod / 16),
		Driver: retryutils.NewExponentialDriver(cfg.MaxRetryPeriod / 16),
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, cancel := context.WithCancel(context.Background())

	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     cfg.TTLCacheRetention,
		Context: closeContext,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	cache := &Cache{
		cfg:      cfg,
		ttlCache: ttlCache,
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	go cache.update(closeContext, retry)

	return cache, nil
}

// GetScopedRole retrieves a scoped role by name.
func (c *Cache) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.roles.GetScopedRole(ctx, req)
}

// ListScopedRoles returns a paginated list of scoped roles.
func (c *Cache) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.roles.ListScopedRoles(ctx, req)
}

// GetScopedRoleAssignment retrieves a scoped role assignment by name.
func (c *Cache) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.assignments.GetScopedRoleAssignment(ctx, req)
}

// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
func (c *Cache) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.assignments.ListScopedRoleAssignments(ctx, req)
}

// PopulatePinnedAssignmentsForUser populates the provided scope pin with all relevant assignments related to the
// given user. The provided pin must already have its Scope field set.
func (c *Cache) PopulatePinnedAssignmentsForUser(ctx context.Context, user string, pin *scopesv1.Pin) error {
	state, err := c.read(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return state.assignments.PopulatePinnedAssignmentsForUser(ctx, user, pin)
}

// Close stops cache background operations and causes future reads to fail. It is safe to call multiple times.
func (c *Cache) Close() error {
	c.cancel()

	// wait for done signal so that all reads with a "happens after" relation to close
	// fail consistently.
	<-c.done
	return nil
}

// update is the main background loop that handles cache setup, update, and retry.
func (c *Cache) update(ctx context.Context, retry retryutils.Retry) {
	defer func() {
		slog.InfoContext(ctx, "scoped access cache closing")
		c.rw.Lock()
		c.closed = true
		c.rw.Unlock()
		close(c.done)
	}()

	for {
		err := c.fetchAndWatch(ctx, retry)
		if ctx.Err() != nil {
			return
		}

		slog.WarnContext(ctx, "scoped access cache failed", "error", err)

		waitStart := time.Now()
		select {
		case <-retry.After():
			retry.Inc()
			slog.InfoContext(ctx, "attempting re-init of scoped access cache after delay", "delay", time.Since(waitStart))
		case <-ctx.Done():
			return
		}
	}
}

// fetchAndWatch attempts to establish a watcher with the upstream events service, populate the cache
// state, and process changes as they come in.
func (c *Cache) fetchAndWatch(ctx context.Context, retry retryutils.Retry) error {
	watcher, err := c.cfg.Events.NewWatcher(ctx, types.Watch{
		Name: "scoped-access-cache",
		Kinds: []types.WatchKind{
			{
				Kind: scopedaccess.KindScopedRole,
			},
			{
				Kind: scopedaccess.KindScopedRoleAssignment,
			},
		},
	})
	if err != nil {
		return trace.Errorf("failed to create watcher: %w", err)
	}

	defer watcher.Close()

	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-watcher.Done():
		if err := watcher.Error(); err != nil {
			// watcher errors are expected if the watcher is closed before init completes.
			return trace.Errorf("watcher failed while waiting for init event: %w", err)
		}
		return trace.Errorf("watcher failed while waiting for init event")
	case <-time.After(retryutils.SeventhJitter(time.Minute)):
		return trace.Errorf("timed out waiting for init event from watcher")
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}

	fetchStart := time.Now()
	state, err := c.fetch(ctx)
	if err != nil {
		return trace.Errorf("failed to fetch initial state: %w", err)
	}

	slog.InfoContext(ctx, "scoped access cache fetched initial state", "elapsed", time.Since(fetchStart))

	c.rw.Lock()
	c.state = state
	c.ok = true
	c.rw.Unlock()

	slog.InfoContext(ctx, "scoped access cache successfully initialized")
	retry.Reset()

	// start processing and applying changes
	for {
		select {
		case event := <-watcher.Events():
			if err := processEvent(ctx, state, event); err != nil {
				return trace.Errorf("failed to process event: %w", err)
			}
		case <-watcher.Done():
			if err := watcher.Error(); err != nil {
				// watcher errors are expected if the watcher is closed before init completes.
				return trace.Errorf("watcher failed during event processing: %w", err)
			}
			return trace.Errorf("watcher failed during event processing")
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// processEvent attempts to update the provided cache state with the given event.
func processEvent(ctx context.Context, state state, event types.Event) error {
	switch event.Type {
	case types.OpPut:
		switch item := event.Resource.(type) {
		case types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]:
			if err := state.roles.Put(item.UnwrapT()); err != nil {
				return trace.Errorf("failed to put scoped role %q: %w", item.UnwrapT().GetMetadata().GetName(), err)
			}
		case types.Resource153UnwrapperT[*scopedaccessv1.ScopedRoleAssignment]:
			if err := state.assignments.Put(item.UnwrapT()); err != nil {
				return trace.Errorf("failed to put scoped role assignment %q: %w", item.UnwrapT().GetMetadata().GetName(), err)
			}
		default:
			return trace.BadParameter("unexpected resource type %T in put event", event.Resource)
		}
	case types.OpDelete:
		switch event.Resource.GetKind() {
		case scopedaccess.KindScopedRole:
			state.roles.Delete(event.Resource.GetName())
		case scopedaccess.KindScopedRoleAssignment:
			state.assignments.Delete(event.Resource.GetName())
		default:
			return trace.BadParameter("unexpected resource kind %q in event delete event", event.Resource.GetKind())
		}
	default:
		slog.WarnContext(ctx, "scoped access cache skipping unexpected event type", "event_type", event.Type)
		return nil
	}
	return nil
}

// read gets a read-ready cache state suitable for use in serving reads. the underlying state may
// be the actual primary cache state, or a ttl-cached image if the primary is unavailable.
func (c *Cache) read(ctx context.Context) (state, error) {
	c.rw.RLock()
	primary, ok, closed := c.state, c.ok, c.closed
	c.rw.RUnlock()

	if closed {
		// theoretically there's nothing wrong with reading *immediately* after close since cache reads are async/trailing
		// anyhow, but allowing reads of a closed cache might mask more serious bugs so its better to fail fast.
		return state{}, trace.Errorf("scoped access cache is closed")
	}

	if ok {
		// the primary cache is available, return it immediately.
		return primary, nil
	}

	// the cache is not ready, load a frozen readonly copy via ttl cache
	temp, err := utils.FnCacheGet(ctx, c.ttlCache, "access-cache", func(ctx context.Context) (state, error) {
		return c.fetch(ctx)
	})

	// primary may have been concurrently loaded. prefer using it if so.
	c.rw.RLock()
	primary, ok = c.state, c.ok
	c.rw.RUnlock()

	if ok {
		return primary, nil
	}

	return temp, trace.Wrap(err)
}

// fetch loads all currently available roles and assignments from the upstream and builds a cache state.
func (c *Cache) fetch(ctx context.Context) (state, error) {
	roleCache := roles.NewRoleCache()

	for role, err := range StreamRoles(ctx, c.cfg.Reader) {
		if err != nil {
			return state{}, trace.Wrap(err)
		}

		if err := roleCache.Put(role); err != nil {
			return state{}, trace.Wrap(err)
		}
	}

	assignmentCache := assignments.NewAssignmentCache()

	for assignment, err := range StreamAssignments(ctx, c.cfg.Reader) {
		if err != nil {
			return state{}, trace.Wrap(err)
		}

		if err := assignmentCache.Put(assignment); err != nil {
			return state{}, trace.Wrap(err)
		}
	}

	return state{
		roles:       roleCache,
		assignments: assignmentCache,
	}, nil
}
