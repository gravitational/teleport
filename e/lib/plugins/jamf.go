package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/services"
)

// jamfInstanceFactory creates a Jamf service based on the plugin specification.
func jamfInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	username, password := deps.staticCredentials[0].GetBasicAuth()
	if username == "" || password == "" {
		return nil, trace.BadParameter("username or password empty")
	}

	jamfSettings := plugin.Spec.GetJamf()
	if jamfSettings == nil {
		return nil, trace.BadParameter("field Spec.Jamf must be present")
	}
	// Assign credential to a new variable so that the parent plugin instance (without credential) remains unchanged.
	jamfSpec := *jamfSettings.JamfSpec
	jamfSpec.Username = username
	jamfSpec.Password = password

	return func() error {
		closeEvent, err := services.JamfPluginInit(deps.lifetime, deps.parentProcess, deps.HTTPClient, deps.statusSink, &jamfSpec)
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
			deps.log.Debugf("Error waiting for JamfStopped event: %v", err)
			return trace.Wrap(err)
		}

		deps.log.Info("Jamf plugin has stopped")
		return nil
	}, nil
}
