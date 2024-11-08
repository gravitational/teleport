package plugins

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// withLeaderLock wraps a plugin’s execution function to ensure that it runs
// exclusively on a single Auth instance at any given time. This is achieved by acquiring
// a distributed lock before execution, which prevents concurrent runs of the same plugin
// across multiple Auth instances.
//
// WARNING: If your plugin has any uses case where HA plugin runtime is required, then
// this handler should not be used.
// For instance if your plugin service is registering a webhook or a gRPC service that needs
// to run on multiple Auth instances for HA, then this handler should not be used.
//
// Please note that plugin manager manages plugin state and plugin manager is responsible
// for plugin lifecycle management. While this function will block the plugin execution on a single
// Auth instance, the plugin manager is responsible for canceling deps.lifetime context to
// due to plugin update or removal or Auth instance shutdown.
func withLeaderLock(handler instanceFactory) instanceFactory {
	return func(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
		pluginFunc, err := handler(ctx, plugin, deps)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Injects deps.lifetime managed by the plugin manager  instead of Teleport Auth
		// process context to ensure that the plugin manager can cancel the runtime lock
		// during plugin updates, removals, or Auth instance shutdown
		return executeWithLeaderLock(deps.lifetime, deps, plugin, pluginFunc), nil
	}
}

func executeWithLeaderLock(ctx context.Context, deps instanceDependencies, plugin *types.PluginV1, pluginFunc func() error) func() error {
	const (
		retryTime = time.Minute * 2
	)

	return func() error {
		lockName := lockNameForPlugin(plugin)
		log := deps.logger.With(
			"lock_name", lockName,
			"auth_server_id", deps.parentProcess.GetAuthServer().ServerID,
			"auth_server_name", deps.parentProcess.GetAuthServer().AuthServiceName,
			"plugin_name", plugin.GetName(),
			"integration_type", plugin.GetType(),
			"retry_time", retryTime,
		)

		for {
			// RunWhileLocked will acquire a lock and run the pluginFunc.
			// If the lock is already acquired by another plugin execution flow
			// The RunWhileLocked will wait for the lock to be released.
			// Ensuring that plugin service is running on a single Auth instance.
			err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
				LockConfiguration: backend.LockConfiguration{
					LockNameComponents: []string{lockName},
					Backend:            deps.parentProcess.GetBackend(),
					TTL:                time.Minute * 3,
					RetryInterval:      time.Minute,
				},
				RefreshLockInterval: time.Minute,
			}, func(ctx context.Context) error {
				log.DebugContext(ctx, "Acquired plugin integration runtime lock.")
				defer log.DebugContext(ctx, "Released plugin integration runtime lock.")
				return trace.Wrap(pluginFunc())
			})

			if err != nil {
				log.DebugContext(ctx, "Plugin instance is in standby mode trying to acquire a lock held by leader, will retry")
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryTime):
			}
		}
	}
}

func lockNameForPlugin(plugin *types.PluginV1) string {
	return fmt.Sprintf("plugin-%s-runtime-lock", plugin.GetName())
}
