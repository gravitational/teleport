package plugins

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins/factory"
)

// afterCacheReady waits for the cache to turn ready before executing the factory.
// This is used to avoid noisy startups because of plugins doing a lot of
// reads while the cache is not populated yet.
// Note: this does not guarantee that the cache is always healthy.
// If the cache breaks later the plugin will still run.
func afterCacheReady(handler factory.Factory) factory.Factory {
	return func(ctx context.Context, plugin *types.PluginV1, deps factory.Dependencies) (factory.Delegate, error) {
		if err := waitForCacheReady(ctx, deps.Client, plugin.GetName()); err != nil {
			return nil, trace.Wrap(err, "waiting for cache for plugin %s", plugin.GetName())
		}
		return handler(ctx, plugin, deps)
	}
}

// waitForCacheReady takes an event source, creates a watcher and waits for the
// OpInit event, meaning the cache is initialized.
// The events source MUST be from the cache.
func waitForCacheReady(ctx context.Context, events types.Events, name string) error {
	w, err := events.NewWatcher(ctx, types.Watch{
		Name: "wait-for-cache-plugin-" + name,
		Kinds: []types.WatchKind{
			// We need to watch any kind, as we are in the plugin manager
			// we expect to be able to watch plugins.
			{Kind: types.KindPlugin},
		},
	})
	if err != nil {
		return trace.Wrap(err, "initializing watcher")
	}

	defer w.Close()

	select {
	case e := <-w.Events():
		if e.Type != types.OpInit {
			return trace.BadParameter("unexpected event type %q, expecting opinit", e.Type)
		}
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "context closed")
	case <-w.Done():
		err := w.Error()
		if err != nil {
			return trace.Wrap(w.Error(), "watcher closed with error")
		}
		// we handle the case where err is nil separately because trace.Wrap(nil, "foo") returns nil
		return trace.CompareFailed("watcher closed without error")
	}

	return nil
}
