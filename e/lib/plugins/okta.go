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

	oktaAPITokenCred, err := okta.SelectAPIToken(deps.staticCredentials)
	if err != nil {
		if trace.IsNotFound(err) {
			if oktaSpec.SyncSettings.SyncUsers || oktaSpec.SyncSettings.SyncAccessLists {
				return nil, trace.Wrap(err)
			}
			// There is no API token to call API and only SCIM integration was enabled.
			// SCIM updates propagated to Teleport by Okta happens only when groups/users are updated in Okta
			// Report RUNNING status and not really emitting plugin status by okta client during calling okta API.
			return func() error {
				if err := deps.statusSink.Emit(ctx, &types.PluginStatusV1{
					Code: types.PluginStatusCode_RUNNING,
				}); err != nil {
					deps.log.WithError(err).Error("Failed to emit status")
				}
				<-deps.lifetime.Done()
				return nil
			}, nil
		}
		return nil, trace.Wrap(err, "selecting okta credentials")
	}

	oktaAPIToken := oktaAPITokenCred.GetAPIToken()
	if oktaAPIToken == "" {
		return nil, trace.BadParameter("api token is empty")
	}

	// TODO: Propagate license changes to Okta hosted plugin runtime.
	// Currently, if license gets upgraded, okta service will still be
	// running with stale settings (unless it was restarted).
	oktaSpec.SyncSettings.SyncUsers = oktaSpec.SyncSettings.SyncUsers && modules.GetModules().Features().IGSEnabled()
	return func() error {
		closeEvent := services.InitOktaPlugin(deps.lifetime,
			services.OktaPluginPrams{
				Process:              deps.parentProcess,
				PluginStatusSink:     deps.statusSink,
				Settings:             *oktaSpec,
				Token:                oktaAPIToken,
				PluginName:           plugin.GetName(),
				AppGroupSyncDisabled: shouldDisabledAppGroupSync(oktaAPITokenCred),
			},
		)
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

// shouldDisabledAppGroupSync check if app user sync should be disabled.
// This is done by checking the label of the token credentials. If the label
// is set to CredPurposeOktaAPITokenWithSCIMOnlyIntegration, then the app group
// sync should be disabled.
//
// Why not use the proto SyncSettings.AppGroupSyncDisabled field?
// Currently, adding fields to the plugin spec is not backward compatible:
// the jsonPB unmarshaler with missing ignore unknown fields will fail to unmarshal the plugin spec.
// when a new field was added.
func shouldDisabledAppGroupSync(tokenCreds types.PluginStaticCredentials) bool {
	v, ok := tokenCreds.GetLabel(okta.CredPurposeLabel)
	return ok && v == okta.CredPurposeOktaAPITokenWithSCIMOnlyIntegration
}
