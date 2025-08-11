package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/intune/api"
	"github.com/gravitational/teleport/e/lib/services"
)

func intuneInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	sc := deps.staticCredentials[0]
	clientID, clientSecret := sc.GetOAuthClientSecret()

	intuneSettings := plugin.Spec.GetIntune()
	if intuneSettings == nil {
		return nil, trace.BadParameter("field Spec.Intune must be present")
	}

	config := api.Config{
		AppCredentials: api.AppCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Tenant:       intuneSettings.Tenant,
		},
		LoginEndpoint: intuneSettings.LoginEndpoint,
		GraphEndpoint: intuneSettings.GraphEndpoint,
	}
	if err := api.ValidateAppCredentials(config.AppCredentials); err != nil {
		return nil, trace.Wrap(err)
	}

	return func() error {
		closeEvent, err := services.IntunePluginInit(deps.lifetime, deps.parentProcess, deps.HTTPClient, deps.statusSink, config)
		if err != nil {
			return trace.Wrap(err)
		}

		// Wait for the calling context to finish before doing anything else.
		<-deps.lifetime.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = deps.parentProcess.WaitForEvent(eventCtx, closeEvent)
		if err != nil {
			deps.logger.DebugContext(ctx, "Error waiting for Intune event", "error", err)
			return trace.Wrap(err)
		}

		deps.logger.InfoContext(ctx, "Intune plugin has stopped")
		return nil
	}, nil
}
