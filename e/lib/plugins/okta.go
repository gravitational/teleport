package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/services"
)

// oktaInstanceFactory will create Okta services based on the plugin specification.
func oktaInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	oktaSpec := plugin.Spec.GetOkta()
	if oktaSpec == nil {
		return nil, trace.BadParameter("field Spec.Okta must be present")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	staticToken := deps.staticCredentials[0].GetAPIToken()
	if staticToken == "" {
		return nil, trace.BadParameter("api token is empty")
	}

	return func() error {
		closeEvent := services.InitOktaPlugin(deps.lifetime, deps.parentProcess, deps.statusSink, *oktaSpec, staticToken, plugin.GetName())

		// wait for the calling context to finish before doing anything else.
		<-deps.lifetime.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := deps.parentProcess.WaitForEvent(eventCtx, closeEvent)

		if err != nil {
			deps.log.Debugf("Error waiting for OktaStopped event: %v", err)
			return trace.Wrap(err)
		}
		deps.log.Info("Okta plugin has stopped")
		return nil
	}, nil
}
