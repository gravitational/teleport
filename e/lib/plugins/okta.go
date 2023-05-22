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

	bearerToken := plugin.Credentials.GetBearerToken().Token

	return func() error {
		closeEvent := services.InitOktaPlugin(ctx, deps.parentProcess, oktaSpec.OrgUrl, bearerToken, plugin.GetName())

		// wait for the calling context to finish before doing anything else.
		<-ctx.Done()

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
