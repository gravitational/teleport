package structured

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// DefaultSupportCacheTTL is the default lifetime of a cached "native structured output unsupported" verdict before the
// native API is re-probed.
const DefaultSupportCacheTTL = 24 * time.Hour

// SupportCache remembers, per (provider key, schema), whether a provider's native structured output API has been
// observed not to work, so the provider can skip re-attempting (and paying for) a failed native request.
//
// The provider key is opaque to the cache: it is whatever identifies the endpoint whose support is being remembered —
// the model ID for Bedrock, or the base URL plus model for an OpenAI-compatible endpoint, since the same model can
// support the native API at one endpoint but not another. Sharing one cache across the providers built for each
// summarization lets endpoints of unknown capability avoid re-probing on every session.
//
// Verdicts are keyed per (key, schema) because a provider can reject one schema as incompatible while supporting the
// native API for others. Verdicts expire after a TTL so the native API is re-probed periodically, letting an endpoint
// that later gains support, or a verdict poisoned by a one-off error, recover without a process restart.
type SupportCache struct {
	mu    sync.RWMutex
	clock clockwork.Clock
	ttl   time.Duration

	// unsupportedUntil maps a (key, schema) pair to the expiry time of its learned "native unsupported" verdict.
	unsupportedUntil map[string]time.Time
}

// NewSupportCache returns a cache whose verdicts expire after ttl, using clock as its time source.
func NewSupportCache(clock clockwork.Clock, ttl time.Duration) *SupportCache {
	return &SupportCache{
		clock:            clock,
		ttl:              ttl,
		unsupportedUntil: make(map[string]time.Time),
	}
}

// Unsupported reports whether (key, schema) has an unexpired "native unsupported" verdict. A nil cache always reports
// false, so a provider can treat the cache as optional.
func (c *SupportCache) Unsupported(key, schema string) bool {
	if c == nil {
		return false
	}

	c.mu.RLock()
	expiry, ok := c.unsupportedUntil[cacheKey(key, schema)]
	c.mu.RUnlock()

	return ok && c.clock.Now().Before(expiry)
}

// UseNative reports whether the native structured output API should be attempted for (key, schema) given the
// endpoint's known support: a known-supported endpoint always uses native, a known-unsupported one never does,
// and an endpoint of unknown support probes unless a recent failed attempt for (key, schema) is still cached.
func (c *SupportCache) UseNative(support NativeSupport, key, schema string) bool {
	switch support {
	case SupportYes:
		return true
	case SupportNo:
		return false
	default: // SupportUnknown
		return !c.Unsupported(key, schema)
	}
}

// MarkUnsupported records that (key, schema)'s native structured output attempt was rejected, suppressing further native
// attempts for that pair until the TTL elapses. A nil cache is a no-op.
func (c *SupportCache) MarkUnsupported(key, schema string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.unsupportedUntil[cacheKey(key, schema)] = c.clock.Now().Add(c.ttl)
}

// cacheKey composes the map key for a (key, schema) pair. The NUL separator cannot appear in a provider key or schema
// fingerprint, so distinct pairs never collide.
func cacheKey(key, schema string) string {
	return key + "\x00" + schema
}
