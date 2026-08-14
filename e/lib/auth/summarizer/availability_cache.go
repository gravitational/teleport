package summarizer

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1"
)

const defaultAvailabilityCacheTTL = 2 * time.Minute

// AvailabilityCache caches the session search availability state reported by
// the access graph. Both the session summarizer (push path) and the session
// search service (query path) share a single instance so the RPC is issued at
// most once per TTL across both callers.
//
// Permanent non-availability states (NOT_IMPLEMENTED, extension missing) are
// cached for the full TTL. Transient RPC errors are not cached so the next
// call retries immediately.
type AvailabilityCache struct {
	mu           sync.Mutex
	value        accessgraphv1.SessionSearchAvailability
	fetchedAt    time.Time
	ttl          time.Duration
	clock        clockwork.Clock
	clientGetter func() (accessgraphv1.SessionRecordingServiceClient, error)
}

// NewAvailabilityCache returns an [AvailabilityCache] backed by clientGetter.
// A zero ttl defaults to [defaultAvailabilityCacheTTL]. A nil clock defaults
// to the real wall clock.
func NewAvailabilityCache(
	clientGetter func() (accessgraphv1.SessionRecordingServiceClient, error),
	clock clockwork.Clock,
	ttl time.Duration,
) (*AvailabilityCache, error) {
	if ttl == 0 {
		ttl = defaultAvailabilityCacheTTL
	}
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	if clientGetter == nil {
		return nil, trace.BadParameter("clientGetter is required")
	}
	return &AvailabilityCache{
		clientGetter: clientGetter,
		ttl:          ttl,
		clock:        clock,
	}, nil
}

// Get returns the cached availability state, refreshing from the access graph
// when the TTL has elapsed. A NOT_IMPLEMENTED result is cached when the access
// graph is not configured or does not implement the RPC. Transient RPC errors
// are returned to the caller without caching.
func (c *AvailabilityCache) Get(ctx context.Context) (accessgraphv1.SessionSearchAvailability, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.fetchedAt.IsZero() && c.clock.Now().Sub(c.fetchedAt) < c.ttl {
		return c.value, nil
	}

	client, err := c.clientGetter()
	if err != nil {
		// Access graph is not configured; treat as permanently not implemented
		// and cache so we don't re-evaluate on every call.
		c.setUnlocked(accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED)
		return c.value, nil
	}

	resp, err := client.IsSessionSearchEnabled(ctx, &accessgraphv1.IsSessionSearchEnabledRequest{})
	switch {
	case status.Code(err) == codes.Unimplemented:
		// Older access graph that predates the RPC; cache and treat as not implemented.
		c.setUnlocked(accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED)
		return c.value, nil
	case err != nil:
		// Transient error; don't cache so the next call retries.
		return accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED, trace.Wrap(err)
	}

	c.setUnlocked(resp.GetAvailability())
	return c.value, nil
}

func (c *AvailabilityCache) setUnlocked(v accessgraphv1.SessionSearchAvailability) {
	c.value = v
	c.fetchedAt = c.clock.Now()
}
