package provider

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/middleware"
	"github.com/gravitational/teleport/e/lib/scim/service/provider/generic"
	"github.com/gravitational/teleport/e/lib/scim/service/provider/okta"
)

// pluginHandlerFunc defines the function signature for creating resource handlers.
type pluginHandlerFunc func(config common.Config, plugin *types.PluginV1, resourceType string) (common.ResourceHandler, error)

// pluginHandlers maps plugin types to their corresponding handler creation functions.
var pluginHandlers = map[types.PluginType]pluginHandlerFunc{
	types.PluginTypeOkta: okta.New,
	types.PluginTypeSCIM: generic.New,
}

// CreateHandlerForPlugin creates a resource handler for the given plugin and wraps it
// with distributed locking middleware to handle concurrent PATCH operations safely.
func CreateHandlerForPlugin(plugin types.Plugin, config common.Config, resourceType string) (common.ResourceHandler, error) {
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("expected plugin to be of type PluginV1, got %T", plugin)
	}

	createFn, ok := pluginHandlers[plugin.GetType()]
	if !ok {
		return nil, trace.BadParameter("unsupported plugin type: %v", plugin.GetType())
	}

	handler, err := createFn(config, pluginV1, resourceType)
	if err != nil {
		return nil, trace.Wrap(err, "creating resource handler for plugin %q", plugin.GetName())
	}

	loggingMiddlewareConfig := middleware.LoggingMiddlewareConfig{
		Log: config.Logger,
	}
	loggingMiddleware, err := middleware.NewLoggingMiddleware(loggingMiddlewareConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating logging middleware")
	}

	// Create a semaphore-based locker for distributed locking across auth servers
	semLock, err := common.NewSemaphoreLocker(config.Semaphore, common.WithClock(config.Clock))
	if err != nil {
		return nil, trace.Wrap(err, "creating semaphore locker")
	}

	// Create lock middleware to serialize PATCH operations per resource
	lockMiddleware, err := middleware.NewLockMiddleware(middleware.LockMiddlewareConfig{
		Locker:     semLock,
		Log:        config.Logger,
		PluginName: plugin.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating lock middleware")
	}

	// Create middleware chain and wrap the handler
	chain := middleware.NewMiddlewareChain(
		loggingMiddleware,
		lockMiddleware,
	)
	return chain.Wrap(handler), nil
}
