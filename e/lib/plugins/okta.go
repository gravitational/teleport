package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

// oktaInstanceFactory will create Okta services based on the plugin specification.
func oktaInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	oktaSpec := plugin.Spec.GetOkta()
	if oktaSpec == nil {
		return nil, trace.BadParameter("field Spec.Okta must be present")
	}

	scimEnabled, err := isOktaSCIMEnabled(deps)
	if err != nil {
		return nil, trace.Wrap(err, "checking if SCIM support is enabled")
	}

	oktaCredsProvider, creds, err := okta.SelectAuthProviderStaticCredentials(ctx, okta.ParamSelectAuthProviderStaticCredentials{
		StaticCredentials: deps.staticCredentials,
		Auth:              deps.parentProcess.GetAuthServer().Cache,
		CAKeyStore:        deps.parentProcess.GetAuthServer().GetKeyStore(),
		Clock:             deps.parentProcess.Clock,
	})
	if trace.IsNotFound(err) {
		if oktaSpec.SyncSettings.SyncUsers {
			return nil, trace.BadParameter("user sync enabled but, Okta credentials missing")
		}
		if oktaSpec.SyncSettings.SyncAccessLists {
			return nil, trace.BadParameter("Access Lists sync enabled but, Okta credentials missing")
		}
		// This is either:
		//   - SSO-only integration
		//   - SCIM-only integration without credentials
		// Report status as running without starting plugin service.
		return func() error {
			if scimEnabled {
				deps.logger.InfoContext(ctx, "SCIM-only integration. Updating plugin status, without starting the plugin")
			} else {
				deps.logger.InfoContext(ctx, "SSO-only integration. Updating plugin status, without starting the plugin")
			}
			// DisableSyncAppGroups is false by default, let's flip it so it's reported
			// correctly.
			oktaSpec.SyncSettings.DisableSyncAppGroups = true
			okta.ReportPluginStatus(
				ctx, deps.logger, deps.statusSink, types.PluginStatusCode_RUNNING,
				okta.NewPluginOktaStatus(okta.PluginOktaStatusParams{
					SyncSettings: *oktaSpec.SyncSettings,
					ScimEnabled:  scimEnabled,
				}),
			)
			<-deps.lifetime.Done()
			return nil
		}, nil
	} else if err != nil {
		return nil, trace.Wrap(err, "selecting okta credentials")
	}

	// TODO: Propagate license changes to Okta hosted plugin runtime.
	// Currently, if license gets upgraded, okta service will still be
	// running with stale settings (unless it was restarted).
	oktaSpec.SyncSettings.SyncUsers = oktaSpec.SyncSettings.SyncUsers && modules.GetModules().Features().GetEntitlement(entitlements.OktaUserSync).Enabled
	oktaSpec.SyncSettings.DisableSyncAppGroups = oktaSpec.SyncSettings.DisableSyncAppGroups || shouldDisableAppGroupSync(creds)
	return func() error {
		closeEvent := services.InitOktaPlugin(deps.lifetime,
			services.OktaPluginPrams{
				Process:          deps.parentProcess,
				PluginStatusSink: deps.statusSink,
				PluginName:       plugin.GetName(),
				OrgUrl:           oktaSpec.OrgUrl,
				AuthProvider:     oktaCredsProvider,
				SyncSettings:     *oktaSpec.SyncSettings,
				SCIMEnabled:      scimEnabled,
			},
		)
		// wait for the calling context to finish before doing anything else.
		<-deps.lifetime.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := deps.parentProcess.WaitForEvent(eventCtx, closeEvent)

		if err != nil {
			deps.logger.DebugContext(ctx, "Error waiting for OktaStopped event", "error", err)
			return trace.Wrap(err)
		}
		deps.logger.InfoContext(ctx, "Okta plugin has stopped")
		return nil
	}, nil
}

func isOktaSCIMEnabled(deps instanceDependencies) (bool, error) {
	// Having a SCIM bearer token set implies that SCIM is enabled for this
	// plugin instance.

	enabled := true
	if _, err := okta.SelectSCIMToken(deps.staticCredentials); err != nil {
		if !trace.IsNotFound(err) {
			return false, trace.Wrap(err, "querying for SCIM credentials")
		}
		deps.logger.InfoContext(context.Background(), "No SCIM credential supplied - SCIM disabled")
		enabled = false
	}
	return enabled, nil
}

// shouldDisableAppGroupSync check if app user sync should be disabled.
// This is done by checking the label of the token credentials. If the label
// is set to CredPurposeOktaAPITokenWithSCIMOnlyIntegration, then the app group
// sync should be disabled.
//
// Why not use the proto SyncSettings.AppGroupSyncDisabled field?
// Currently, adding fields to the plugin spec is not backward compatible:
// the jsonPB unmarshaler with missing ignore unknown fields will fail to unmarshal the plugin spec.
// when a new field was added.
func shouldDisableAppGroupSync(tokenCreds types.PluginStaticCredentials) bool {
	v, ok := tokenCreds.GetLabel(common.CredPurposeLabel)
	return ok && v == common.CredPurposeOktaAPITokenWithSCIMOnlyIntegration
}
