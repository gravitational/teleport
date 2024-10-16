package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// mappedCollection is an extension of `genericCollection` that allows the
// runtime resource to be of a different type to the underlying cache resource
type mappedCollection[RuntimeT any, CacheT any, R any, E executor[RuntimeT, R]] struct {
	inner  genericCollection[RuntimeT, R, E]
	mapper func(CacheT) RuntimeT
}

// fetch implements collection
func (g *mappedCollection[RuntimeT, CacheT, R, _]) fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error) {
	return g.inner.fetch(ctx, cacheOK)
}

// processEvent implements collection
func (g *mappedCollection[RuntimeT, CacheT, R, _]) processEvent(ctx context.Context, event types.Event) error {
	cache := g.inner.cache
	exec := g.inner.exec

	switch event.Type {
	case types.OpDelete:
		if err := exec.delete(ctx, g.inner.cache, event.Resource); err != nil {
			if !trace.IsNotFound(err) {
				cache.Logger.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}

	case types.OpPut:
		var resource CacheT
		var ok bool
		switch r := event.Resource.(type) {
		case types.Resource153Unwrapper:
			inner := r.Unwrap()
			resource, ok = inner.(CacheT)
			if !ok {
				return trace.BadParameter("unexpected wrapped type %T (expected %T)", inner, resource)
			}

		case CacheT:
			resource = r

		default:
			return trace.BadParameter("unexpected type %T (expected %T)", event.Resource, resource)
		}

		if err := exec.upsert(ctx, cache, g.mapper(resource)); err != nil {
			return trace.Wrap(err)
		}

	default:
		cache.Logger.WithField("event", event.Type).Warn("Skipping unsupported event type.")
	}
	return nil
}

// watchKind implements collection
func (g *mappedCollection[RuntimeT, ResourceT, R, _]) watchKind() types.WatchKind {
	return g.inner.watch
}

// genericCollection obtains the reader object from the executor based on the provided health status of the cache.
// Note that cacheOK set to true means that cache is overall healthy and the collection was confirmed as supported.
func (c *mappedCollection[RuntimeT, ResourceT, R, _]) getReader(cacheOK bool) R {
	return c.inner.exec.getReader(c.inner.cache, cacheOK)
}

var _ collection = (*mappedCollection[types.Resource, types.Resource, any, executor[types.Resource, any]])(nil)
