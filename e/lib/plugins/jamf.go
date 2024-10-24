package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// jamfInstanceFactory creates a Jamf service based on the plugin specification.
func jamfInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	sc := deps.staticCredentials[0]

	// username+password is set for plugins created before the introduction of
	// clientID+clientSecret.
	username, password := sc.GetBasicAuth()
	clientID, clientSecret := sc.GetOAuthClientSecret()
	if (username == "" || password == "") && (clientID == "" || clientSecret == "") {
		return nil, trace.BadParameter("credentials must be present")
	}

	jamfSettings := plugin.Spec.GetJamf()
	if jamfSettings == nil || jamfSettings.JamfSpec == nil {
		return nil, trace.BadParameter("field Spec.Jamf must be present")
	}

	// Assign credential to a new variable so that the parent plugin instance (without credential) remains unchanged.
	jamfSpec := *jamfSettings.JamfSpec

	creds := &servicecfg.JamfCredentials{
		Username:     username,
		Password:     password,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	return func() error {
		closeEvent, err := services.JamfPluginInit(deps.lifetime, deps.parentProcess, deps.HTTPClient, deps.statusSink, &jamfSpec, creds)
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
			deps.logger.DebugContext(ctx, "Error waiting for JamfStopped event", "error", err)
			return trace.Wrap(err)
		}

		deps.logger.InfoContext(ctx, "Jamf plugin has stopped")
		return nil
	}, nil
}
