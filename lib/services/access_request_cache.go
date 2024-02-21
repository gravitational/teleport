package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/sortcache"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

const (
	requestID    = "ID"
	requestState = "State"
)

type AccessRequestCacheConfig struct {
	// Clock is a clock for time-related operation.
	Clock clockwork.Clock
	// Events is an event system client.
	Events types.Events
	// Getter is an access request getter client.
	Getter AccessRequestGetter
	// Context is an optional parent context for the cache's
	// internal close context.
	Context context.Context
}

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

	if c.Context == nil {
		c.Context = context.Background()
	}

	return nil
}

type AccessRequestCache struct {
	rw           sync.RWMutex
	cfg          AccessRequestCacheConfig
	primaryCache *sortcache.SortCache[*types.AccessRequestV3]
	ttlCache     *utils.FnCache
	initC        chan struct{}
	closeContext context.Context
	cancel       context.CancelFunc
}

func NewAccessRequestCache(cfg AccessRequestCacheConfig) (*AccessRequestCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.Context)

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
		Component: "access-request-cache",
		Client:    cfg.Events,
	}); err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	return c, nil
}

func (c *AccessRequestCache) ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error) {
	if req.Filter == nil {
		req.Filter = &types.AccessRequestFilter{}
	}

	if req.Filter.ID != "" {
		// important special case: single-request lookups must always be forwarded to the real backend to avoid race conditions whereby
		// stale cache state causes spurious errors due to users trying to utilize an access request immediately after it gets approved.
		return c.cfg.Getter.ListAccessRequests(ctx, req)
	}

	panic("TODO: page size")

	cache, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var index string
	switch req.Sort {
	case proto.AccessRequestSort_DEFAULT, proto.AccessRequestSort_CREATED:
		index = requestID
	case proto.AccessRequestSort_STATE:
		index = requestState
	default:
		return nil, trace.BadParameter("unsupported access request sort index '%v'", req.Sort)
	}

	if !cache.HasIndex(index) {
		// this case would be a fairly trivial programming error, but its best to give it
		// a friendly error message.
		return nil, trace.Errorf("access request cache was not configured with sort index %q (this is a bug)", index)
	}

	traverse := cache.Ascend
	if req.Descending {
		traverse = cache.Descend
	}

	limit := int(req.Limit)

	// perform the traversel until we've seen all items or fill the page
	var rsp proto.ListAccessRequestsResponse
	traverse(index, req.StartKey, "", func(r *types.AccessRequestV3) bool {
		if !req.Filter.Match(r) {
			return true
		}

		c := r.Copy()
		cr, ok := c.(*types.AccessRequestV3)
		if !ok {
			log.Warnf("%T.Clone returned unexpected type %T (this is a bug).", r, c)
			return true
		}

		rsp.AccessRequests = append(rsp.AccessRequests, cr)

		// halt when we have Limit+1 items so that we can create a
		// correct 'NextKey'.
		return len(rsp.AccessRequests) <= limit
	})

	if len(rsp.AccessRequests) > limit {
		rsp.NextKey = cache.KeyOf(index, rsp.AccessRequests[limit])
		rsp.AccessRequests = rsp.AccessRequests[:limit]
	}

	return &rsp, nil
}

func (c *AccessRequestCache) fetch(ctx context.Context) (*sortcache.SortCache[*types.AccessRequestV3], error) {
	cache := sortcache.New(sortcache.Config[*types.AccessRequestV3]{
		Indexes: map[string]func(*types.AccessRequestV3) string{
			requestID: func(req *types.AccessRequestV3) string {
				// since requestID is equivalent to the DEFAULT sort index (i.e. the sort index of the backend),
				// it is preferable to keep its format equivalent to the format of the NextKey/StartKey values
				// expected by the backend implementation of ListResources.
				return req.GetName()
			},
			requestState: func(req *types.AccessRequestV3) string {
				return fmt.Sprintf("%s/%s", req.GetState(), req.GetName())
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
				log.Warnf("AccessRequest %q conflicted with %d other requests during cache fetch. This is a bug and may result in requests not appearing in UI/CLI correctly.", r.GetName(), evicted)
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
func (c *AccessRequestCache) read(ctx context.Context) (*sortcache.SortCache[*types.AccessRequestV3], error) {
	c.rw.RLock()
	primary := c.primaryCache
	c.rw.RUnlock()

	if primary != nil {
		return primary, nil
	}

	temp, err := utils.FnCacheGet(ctx, c.ttlCache, "access-request-cache", func(ctx context.Context) (*sortcache.SortCache[*types.AccessRequestV3], error) {
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

func (c *AccessRequestCache) resourceKinds() []types.WatchKind {
	return []types.WatchKind{
		{
			Kind: types.KindAccessRequest,
		},
	}
}
func (c *AccessRequestCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	cache, err := c.fetch(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	c.rw.Lock()
	defer c.rw.Unlock()
	c.primaryCache = cache
	return nil
}

func (c *AccessRequestCache) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	switch event.Type {
	case types.OpPut:
		req, ok := event.Resource.(*types.AccessRequestV3)
		if !ok {
			log.Warnf("Unexpected resource type %T in event (expected %T)", event.Resource, req)
			return
		}
		if evicted := c.primaryCache.Put(req); evicted > 1 {
			// this warning, if it appears, means that we configured our indexes incorrectly and one access request is overwriting another.
			// the most likely explanation is that one of our indexes is missing the request id suffix we typically use.
			log.Warnf("Processing of put event for request %q resulted in multiple cache evictions (this is a bug).", req.GetName())
		}
	case types.OpDelete:
		c.primaryCache.Delete(requestID, event.Resource.GetName())
	default:
		log.Warnf("Unexpected event variant: %+v", event)
	}
}

func (c *AccessRequestCache) notifyStale() {
	c.rw.Lock()
	defer c.rw.Unlock()
	if c.primaryCache == nil {
		return
	}
	c.primaryCache = nil
	c.initC = make(chan struct{})
}

func (c *AccessRequestCache) initializationChan() <-chan struct{} {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.initC
}
