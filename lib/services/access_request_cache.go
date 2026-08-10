/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type accessRequestCacheIndex string

const (
	// accessRequestID is the name of the sort index used for sorting request by ID (equivalent to proto.AccessRequestSort_DEFAULT since
	// access requests currently default to being sorted by ID in the backend).
	accessRequestID accessRequestCacheIndex = "ID"
	// accessRequestCreated is the name of the sort index used for sorting requests by creation time (this is typically the sort order
	// used in user interfaces, since most users that want to view requests want to see the most recent requests specifically).
	accessRequestCreated accessRequestCacheIndex = "Created"
	// accessRequestState is the name of the sort index used for sorting requests by their current state (pending, approved, etc).
	accessRequestState accessRequestCacheIndex = "State"
	// accessRequestUser is the name of the sort index used for sorting requests by the person who created the request.
	accessRequestUser accessRequestCacheIndex = "User"
)

// AccessRequestCacheConfig holds the configuration parameters for an [AccessRequestCache].
type AccessRequestCacheConfig struct {
	// Clock is a clock for time-related operation.
	Clock clockwork.Clock
	// Events is an event system client.
	Events types.Events
	// Getter is an access request getter client.
	Getter AccessRequestGetter
	// MaxRetryPeriod is the maximum retry period on failed watches.
	MaxRetryPeriod time.Duration
}

// CheckAndSetDefaults valides the config and provides reasonable defaults for optional fields.
func (c *AccessRequestCacheConfig) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Events == nil {
		return trace.BadParameter("access request cache config missing event system client")
	}

	if c.Getter == nil {
		return trace.BadParameter("access request cache config missing access request getter")
	}

	return nil
}

// AccessRequestCache is a custom cache for access requests that offers custom sort indexes not
// supported by the standard backend implementation. As with all caches, the state observed during
// reads may be slightly outdated. There is a builtin fallback that always routes requests for a
// single specific access request (specified by ID) to the real backend, to avoid outdated single-resource
// reads. Usecases that need perfectly up to date information (e.g. loading an access request in order
// to generate a certificate) should always load the desired request by ID for this reason.
type AccessRequestCache struct {
	rw           sync.RWMutex
	cfg          AccessRequestCacheConfig
	primaryCache *sortcache.SortCache[*types.AccessRequestV3, accessRequestCacheIndex]
	ttlCache     *utils.FnCache
	initC        chan struct{}
	initOnce     sync.Once
	closeContext context.Context
	cancel       context.CancelFunc
	// onInit is a callback used in tests to detect
	// individual initializations.
	onInit func()
}

// NewAccessRequestCache sets up a new [AccessRequestCache] instance based on the supplied
// configuration. The cache is initialized asychronously in the background, so while it is
// safe to read from it immediately, performance is better after the cache properly initializes.
func NewAccessRequestCache(cfg AccessRequestCacheConfig) (*AccessRequestCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context: ctx,
		TTL:     15 * time.Second,
		Clock:   cfg.Clock,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	c := &AccessRequestCache{
		cfg:          cfg,
		ttlCache:     ttlCache,
		initC:        make(chan struct{}),
		closeContext: ctx,
		cancel:       cancel,
	}

	if _, err := newResourceWatcher(ctx, c, ResourceWatcherConfig{
		Component:      "access-request-cache",
		Client:         cfg.Events,
		MaxRetryPeriod: cfg.MaxRetryPeriod,
	}); err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	return c, nil
}

// ListAccessRequests is an access request getter with pagination and sorting options.
func (c *AccessRequestCache) ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error) {
	rsp, err := c.ListMatchingAccessRequests(ctx, req, func(_ *types.AccessRequestV3) bool {
		return true
	})

	return rsp, trace.Wrap(err)
}

// ListMatchingAccessRequests is equivalent to ListAccessRequests except that it adds the ability to provide an arbitrary matcher function. This method
// should be preferred when using custom filtering (e.g. access-controls), since the paginations keys used by the access request cache are non-standard.
func (c *AccessRequestCache) ListMatchingAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest, match func(*types.AccessRequestV3) bool) (*proto.ListAccessRequestsResponse, error) {
	const maxPageSize = 16_000

	if req.Filter == nil {
		req.Filter = &types.AccessRequestFilter{}
	}

	if req.Filter.ID != "" {
		// important special case: single-request lookups must always be forwarded to the real backend to avoid race conditions whereby
		// stale cache state causes spurious errors due to users trying to utilize an access request immediately after it gets approved.
		rsp, err := c.cfg.Getter.ListAccessRequests(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// fallback doesn't apply the match function, so we need to apply it manually here.
		matched := rsp.AccessRequests[:0]
		for _, req := range rsp.AccessRequests {
			if !match(req) {
				continue
			}
			matched = append(matched, req)
		}
		rsp.AccessRequests = matched
		return rsp, nil
	}

	if req.Limit == 0 {
		req.Limit = apidefaults.DefaultChunkSize
	}

	if req.Limit > maxPageSize {
		return nil, trace.BadParameter("page size of %d is too large", req.Limit)
	}

	cache, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var index accessRequestCacheIndex
	switch req.Sort {
	case proto.AccessRequestSort_DEFAULT:
		index = accessRequestID
	case proto.AccessRequestSort_CREATED:
		index = accessRequestCreated
	case proto.AccessRequestSort_STATE:
		index = accessRequestState
	case proto.AccessRequestSort_USER:
		index = accessRequestUser
	default:
		return nil, trace.BadParameter("unsupported access request sort index '%v'", req.Sort)
	}

	if !cache.HasIndex(index) {
		// this case would be a fairly trivial programming error, but its best to give it
		// a friendly error message.
		return nil, trace.Errorf("access request cache was not configured with sort index %q (this is a bug)", index)
	}

	accessRequests := cache.Ascend
	if req.Descending {
		accessRequests = cache.Descend
	}

	limit := int(req.Limit)

	// perform the traversal until we've seen all items or fill the page
	var rsp proto.ListAccessRequestsResponse
	now := time.Now()
	var expired int
	for r := range accessRequests(index, req.StartKey, "") {
		if len(rsp.AccessRequests) == limit {
			rsp.NextKey = cache.KeyOf(index, r)
			break
		}

		if !r.Expiry().IsZero() && now.After(r.Expiry()) {
			expired++
			// skip requests that appear expired. some backends can take up to 48 hours to expired items
			// and access requests showing up past their expiry time is particularly confusing.
			continue
		}
		if !req.Filter.Match(r) || !match(r) {
			continue
		}

		c := r.Copy()
		cr, ok := c.(*types.AccessRequestV3)
		if !ok {
			slog.WarnContext(ctx, "clone returned unexpected type (this is a bug)", "expected", logutils.TypeAttr(r), "got", logutils.TypeAttr(c))
			continue
		}

		rsp.AccessRequests = append(rsp.AccessRequests, cr)
	}

	if expired > 0 {
		// this is a debug-level log since some amount of delay between expiry and backend cleanup is expected, but
		// very large and/or disproportionate numbers of stale access requests might be a symptom of a deeper issue.
		slog.DebugContext(ctx, "omitting expired access requests from cache read", "count", expired)
	}

	return &rsp, nil
}

// fetch configures a sortcache and inserts all currently extant access requests into it. this method is used both
// as the means of setting up the initial primary cache state, and for creating temporary cache states to read from
// when the primary is unhealthy.
func (c *AccessRequestCache) fetch(ctx context.Context) (*sortcache.SortCache[*types.AccessRequestV3, accessRequestCacheIndex], error) {
	cache := sortcache.New(sortcache.Config[*types.AccessRequestV3, accessRequestCacheIndex]{
		Indexes: map[accessRequestCacheIndex]func(*types.AccessRequestV3) string{
			accessRequestID: func(req *types.AccessRequestV3) string {
				// since accessRequestID is equivalent to the DEFAULT sort index (i.e. the sort index of the backend),
				// it is preferable to keep its format equivalent to the format of the NextKey/StartKey values
				// expected by the backend implementation of ListResources.
				return req.GetName()
			},
			accessRequestCreated: func(req *types.AccessRequestV3) string {
				return fmt.Sprintf("%s/%s", req.GetCreationTime().Format(time.RFC3339), req.GetName())
			},
			accessRequestState: func(req *types.AccessRequestV3) string {
				return fmt.Sprintf("%s/%s", req.GetState().String(), req.GetName())
			},
			accessRequestUser: func(req *types.AccessRequestV3) string {
				return fmt.Sprintf("%s/%s", req.GetUser(), req.GetName())
			},
		},
	})

	var req proto.ListAccessRequestsRequest
	for {
		rsp, err := c.cfg.Getter.ListAccessRequests(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range rsp.AccessRequests {
			if evicted := cache.Put(r); evicted != 0 {
				// this warning, if it appears, means that we configured our indexes incorrectly and one access request is overwriting another.
				// the most likely explanation is that one of our indexes is missing the request id suffix we typically use.
				slog.WarnContext(ctx, "conflict during access request fetch (this is a bug and may result in missing requests)", "id", r.GetName(), "evicted", evicted)
			}
		}

		if rsp.NextKey == "" {
			break
		}
		req.StartKey = rsp.NextKey
	}

	return cache, nil
}

// read gets a read-only view into a valid cache state. it prefers reading from the primary cache, but will fallback
// to a periodically reloaded temporary state when the primary state is unhealthy.
func (c *AccessRequestCache) read(ctx context.Context) (*sortcache.SortCache[*types.AccessRequestV3, accessRequestCacheIndex], error) {
	c.rw.RLock()
	primary := c.primaryCache
	c.rw.RUnlock()

	// primary cache state is healthy, so use that. note that we don't protect access to the sortcache itself
	// via our rw lock. sortcaches have their own internal locking.  we just use our lock to protect the *pointer*
	// to the sortcache.
	if primary != nil {
		return primary, nil
	}

	temp, err := utils.FnCacheGet(ctx, c.ttlCache, "access-request-cache", func(ctx context.Context) (*sortcache.SortCache[*types.AccessRequestV3, accessRequestCacheIndex], error) {
		return c.fetch(ctx)
	})

	// primary may have been concurrently loaded. if it was, prefer using that.
	c.rw.RLock()
	primary = c.primaryCache
	c.rw.RUnlock()

	if primary != nil {
		return primary, nil
	}

	return temp, trace.Wrap(err)
}

// --- the below methods implement the resourceCollector interface ---

// resourceKinds is part of the resourceCollector interface and is used to configure the event watcher
// that monitors for access request modifications.
func (c *AccessRequestCache) resourceKinds() []types.WatchKind {
	return []types.WatchKind{
		{
			Kind: types.KindAccessRequest,
		},
	}
}

// getResourcesAndUpdateCurrent is part of the resourceCollector interface and is called one the
// event stream for the cache has been initialized to trigger setup of the initial primary cache state.
func (c *AccessRequestCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	cache, err := c.fetch(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	c.rw.Lock()
	defer c.rw.Unlock()
	c.primaryCache = cache
	c.initOnce.Do(func() {
		close(c.initC)
	})
	if c.onInit != nil {
		c.onInit()
	}
	return nil
}

// SetInitCallback is used in tests that care about cache inits.
func (c *AccessRequestCache) SetInitCallback(cb func()) {
	c.rw.Lock()
	defer c.rw.Unlock()
	c.onInit = cb
}

// processEventsAndUpdateCurrent is part of the resourceCollector interface and is used to update the
// primary cache state when modification events occur.
func (c *AccessRequestCache) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	c.rw.RLock()
	cache := c.primaryCache
	c.rw.RUnlock()

	for _, event := range events {
		switch event.Type {
		case types.OpPut:
			req, ok := event.Resource.(*types.AccessRequestV3)
			if !ok {
				slog.WarnContext(ctx, "unexpected resource type in event", "expected", logutils.TypeAttr(req), "got", logutils.TypeAttr(event.Resource))
				continue
			}
			if evicted := cache.Put(req); evicted > 1 {
				// this warning, if it appears, means that we configured our indexes incorrectly and one access request is overwriting another.
				// the most likely explanation is that one of our indexes is missing the request id suffix we typically use.
				slog.WarnContext(ctx, "request put event resulted in multiple cache evictions (this is a bug)", "id", req.GetName(), "evicted", evicted)
			}
		case types.OpDelete:
			cache.Delete(accessRequestID, event.Resource.GetName())
		default:
			slog.WarnContext(ctx, "unexpected event variant", "op", logutils.StringerAttr(event.Type), "resource", logutils.TypeAttr(event.Resource))
		}
	}
}

// notifyStale is part of the resourceCollector interface and is used to inform
// the access request cache that its view is outdated (presumably due to issues with
// the event stream).
func (c *AccessRequestCache) notifyStale() {
	c.rw.Lock()
	defer c.rw.Unlock()
	if c.primaryCache == nil {
		return
	}
	c.primaryCache = nil
	c.initC = make(chan struct{})
	c.initOnce = sync.Once{}
}

// initializationChan is part of the resourceCollector interface and gets the channel
// used to signal that the accessRequestCache has been initialized.
func (c *AccessRequestCache) initializationChan() <-chan struct{} {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.initC
}

// InitializationChan is part of the resourceCollector interface and gets the channel
// used to signal that the accessRequestCache has been initialized.
func (c *AccessRequestCache) InitializationChan() <-chan struct{} {
	return c.initializationChan()
}

// Close terminates the background process that keeps the access request cache up to
// date, and terminates any inflight load operations.
func (c *AccessRequestCache) Close() error {
	c.cancel()
	return nil
}
