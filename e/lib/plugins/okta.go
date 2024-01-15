package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/lib/modules"
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

	oktaAPITokenCred, err := selectOktaAPIToken(deps.staticCredentials)
	if err != nil {
		return nil, trace.Wrap(err, "selecting okta credentials")
	}

	oktaAPIToken := oktaAPITokenCred.GetAPIToken()
	if oktaAPIToken == "" {
		return nil, trace.BadParameter("api token is empty")
	}

	// TODO: Propagate license changes to Okta hosted plugin runtime.
	// Currently, if license gets upgraded, okta service will still be
	// running with stale settings (unless it was restarted).
	oktaSpec.EnableUserSync = oktaSpec.EnableUserSync && modules.GetModules().Features().IGSEnabled()

	return func() error {
		closeEvent := services.InitOktaPlugin(deps.lifetime, deps.parentProcess, deps.statusSink, *oktaSpec, oktaAPIToken, plugin.GetName())

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

func selectOktaAPIToken(staticCredentials []types.PluginStaticCredentials) (types.PluginStaticCredentials, error) {

	// For now, we'll just choose the first eligible static credential until we
	// have a need for rotation or other complexity.
	for _, cred := range staticCredentials {
		// Older Okta API credentials are not labeled with a purpose, so a cred
		// is considered eligible if it has no purpose label, or a purpose label
		// set to okta.CredPurposeOktaAuth.
		purpose, present := cred.GetLabel(okta.CredPurposeLabel)
		if !present || purpose == okta.CredPurposeOktaAuth {
			return cred, nil
		}
	}

	return nil, trace.NotFound("Okta API token")
}
