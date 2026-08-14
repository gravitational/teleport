package factory

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/services"
)

// NetIQ will create NetIQ services based on the plugin specification.
func NetIQ(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	netIQSpec := plugin.Spec.GetNetIq()
	if netIQSpec == nil {
		return nil, trace.BadParameter("field Spec.NetIQ must be present")
	}

	return func(ctx context.Context) error {
		closeEvent, err := services.NetIQPluginInit(ctx, deps.ParentProcess, deps.StatusSink, netIQSpec, deps.StaticCredentials)
		if err != nil {
			return trace.Wrap(err)
		}

		// wait for the calling context to finish before doing anything else.
		<-ctx.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = deps.ParentProcess.WaitForEvent(eventCtx, closeEvent)
		if err != nil {
			deps.Logger.DebugContext(ctx, "Error waiting for event", "event", closeEvent, "error", err)
			return trace.Wrap(err)
		}

		deps.Logger.InfoContext(ctx, "NetIQ plugin has stopped")
		return nil
	}, nil
}
