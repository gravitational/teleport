package factory

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/intune"
	"github.com/gravitational/teleport/e/lib/intune/api"
	"github.com/gravitational/teleport/e/lib/services"
)

func Intune(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	sc := deps.StaticCredentials[0]
	clientID, clientSecret := sc.GetOAuthClientSecret()

	intuneSettings := plugin.Spec.GetIntune()
	if intuneSettings == nil {
		return nil, trace.BadParameter("field Spec.Intune must be present")
	}

	config := intune.APIConfig{
		AppCredentials: api.AppCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Tenant:       intuneSettings.Tenant,
		},
		LoginEndpoint: intuneSettings.LoginEndpoint,
		GraphEndpoint: intuneSettings.GraphEndpoint,
	}
	if err := config.AppCredentials.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		closeEvent, err := services.IntunePluginInit(ctx, deps.ParentProcess, deps.HTTPClient, deps.StatusSink, config)
		if err != nil {
			return trace.Wrap(err)
		}

		// Wait for the calling context to finish before doing anything else.
		<-ctx.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = deps.ParentProcess.WaitForEvent(eventCtx, closeEvent)
		if err != nil {
			deps.Logger.DebugContext(ctx, "Error waiting for Intune event", "error", err)
			return trace.Wrap(err)
		}

		deps.Logger.InfoContext(ctx, "Intune plugin has stopped")
		return nil
	}, nil
}
