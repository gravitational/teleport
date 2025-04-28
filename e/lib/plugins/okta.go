package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
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

	_, scimEnabled, err := oktaplugin.SelectSCIMTokenHash(deps.staticCredentials)
	if err != nil {
		return nil, trace.Wrap(err, "checking if SCIM support is enabled")
	}
	var oktaAuthProvider oktaapi.AuthProvider
	selectedOktaCreds, err := oktaplugin.SelectOktaCredentials(deps.staticCredentials)
	if err != nil {
		return nil, trace.Wrap(err, "looking up for Okta credentials")
	}
	switch {
	case selectedOktaCreds.OauthClientId != "":
		oktaAuthProvider = oktaapi.NewOauthProviderWithOktaCASigner(ctx, oktaapi.OauthOktaCACredentialsConfig{
			OAuthClientID: selectedOktaCreds.OauthClientId,
			AuthService:   deps.parentProcess.GetAuthServer().Cache,
			CAKeyStore:    deps.parentProcess.GetAuthServer().GetKeyStore(),
			Clock:         deps.parentProcess.Clock,
		})
	case selectedOktaCreds.ApiToken != "":
		oktaAuthProvider = oktaapi.NewSSWSAuthProvider(selectedOktaCreds.ApiToken)
	default:
		return func() error {
			authServer := deps.parentProcess.GetAuthServer()
			connectorInfo, err := authServer.GetSAMLConnector(ctx, oktaSpec.SyncSettings.SsoConnectorId, false)
			if err != nil {
				return trace.Wrap(err)
			}
			status := okta.NewPluginOktaStatus(okta.PluginOktaStatusParams{
				SsoConnector: connectorInfo,
				SyncSettings: *oktaSpec.GetSyncSettings(),
				ScimEnabled:  false,
			})
			if isSyncEnabled(status) {
				err := trace.BadParameter("Okta credentials missing")
				setSyncDetailsError(status, err)
				okta.ReportPluginStatus(ctx, deps.logger, deps.statusSink, types.PluginStatusCode_OTHER_ERROR, status)
				return trace.Wrap(err)
			}
			if scimEnabled {
				deps.logger.InfoContext(ctx, "SCIM-only integration. Updating plugin status, without starting the plugin")
			} else {
				deps.logger.InfoContext(ctx, "SSO-only integration. Updating plugin status, without starting the plugin")
			}
			okta.ReportPluginStatus(ctx, deps.logger, deps.statusSink, types.PluginStatusCode_RUNNING, status)

			<-deps.lifetime.Done()
			return nil
		}, nil
	}

	// TODO: Propagate license changes to Okta hosted plugin runtime.
	// Currently, if license gets upgraded, okta service will still be
	// running with stale settings (unless it was restarted).
	oktaSpec.SyncSettings.SyncUsers = oktaSpec.SyncSettings.SyncUsers && modules.GetModules().Features().GetEntitlement(entitlements.OktaUserSync).Enabled
	oktaSpec.SyncSettings.DisableSyncAppGroups = oktaSpec.SyncSettings.DisableSyncAppGroups || selectedOktaCreds.ApiTokenForSCIMOnly
	return func() error {
		closeEvent := services.InitOktaPlugin(deps.lifetime,
			services.OktaPluginPrams{
				Process:          deps.parentProcess,
				PluginStatusSink: deps.statusSink,
				PluginName:       plugin.GetName(),
				OrgUrl:           oktaSpec.OrgUrl,
				AuthProvider:     oktaAuthProvider,
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

func isSyncEnabled(status *types.PluginOktaStatusV1) bool {
	if status.UsersSyncDetails != nil && status.UsersSyncDetails.Enabled {
		return true
	}
	if status.AppGroupSyncDetails != nil && status.AppGroupSyncDetails.Enabled {
		return true
	}
	if status.AccessListsSyncDetails != nil && status.AccessListsSyncDetails.Enabled {
		return true
	}
	return false
}

func setSyncDetailsError(status *types.PluginOktaStatusV1, err error) {
	if status.UsersSyncDetails != nil && status.UsersSyncDetails.Enabled {
		status.UsersSyncDetails.Error = err.Error()
	}
	if status.AppGroupSyncDetails != nil && status.AppGroupSyncDetails.Enabled {
		status.AppGroupSyncDetails.Error = err.Error()
	}
	if status.AccessListsSyncDetails != nil && status.AccessListsSyncDetails.Enabled {
		status.AccessListsSyncDetails.Error = err.Error()
	}
}
