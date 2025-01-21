package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/services"
)

// netIQInstanceFactory will create NetIQ services based on the plugin specification.
func netIQInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	netIQSpec := plugin.Spec.GetNetIq()
	if netIQSpec == nil {
		return nil, trace.BadParameter("field Spec.NetIQ must be present")
	}

	return func() error {
		closeEvent, err := services.NetIQPluginInit(deps.lifetime, deps.parentProcess, deps.statusSink, netIQSpec, deps.staticCredentials)
		if err != nil {
			return trace.Wrap(err)
		}

		// wait for the calling context to finish before doing anything else.
		<-deps.lifetime.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = deps.parentProcess.WaitForEvent(eventCtx, closeEvent)
		if err != nil {
			deps.logger.DebugContext(ctx, "Error waiting for event", "event", closeEvent, "error", err)
			return trace.Wrap(err)
		}

		deps.logger.InfoContext(ctx, "NetIQ plugin has stopped")
		return nil
	}, nil
}
