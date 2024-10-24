package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/services"
)

// entraIDInstanceFactory will create Entra ID services based on the plugin specification.
func entraIDInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	entraSpec := plugin.Spec.GetEntraId()
	if entraSpec == nil {
		return nil, trace.BadParameter("field Spec.EntraId must be present")
	}

	authServer := deps.parentProcess.GetAuthServer()

	return func() error {
		integration, err := authServer.Services.GetIntegration(ctx, plugin.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		azureSpec := integration.GetAzureOIDCIntegrationSpec()
		if azureSpec == nil {
			return trace.BadParameter("expected %q to be an %q integration, was %q instead", integration.GetName(), types.IntegrationSubKindAzureOIDC, integration.GetSubKind())
		}

		closeEvent, err := services.EntraIDPluginInit(deps.lifetime, deps.parentProcess, deps.statusSink, entraSpec, azureSpec)
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

		deps.logger.InfoContext(ctx, "Entra ID plugin has stopped")
		return nil
	}, nil
}
