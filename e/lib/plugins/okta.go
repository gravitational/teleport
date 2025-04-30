package plugins

import (
	"context"
	"strconv"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
)

// oktaInstanceFactory will create Okta services based on the plugin specification.
func oktaInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	oktaSpec := plugin.Spec.GetOkta()
	if oktaSpec == nil {
		return nil, trace.BadParameter("field Spec.Okta must be present")
	}

	syncEnabled := oktaSpec.GetSyncSettings().GetEnableUserSync()

	_, scimEnabled, err := oktaplugin.SelectSCIMTokenHash(deps.staticCredentials)
	if err != nil {
		return nil, trace.Wrap(err, "checking if SCIM support is enabled")
	}

	// If sync is not enabled (SSO-only/SCIM-only integration) then only report the plugin's
	// status and for the done channel.
	if !syncEnabled {
		return func() error {
			if scimEnabled {
				deps.logger.InfoContext(ctx, "SCIM-only integration. Updating plugin status, without starting the plugin")
			} else {
				deps.logger.InfoContext(ctx, "SSO-only integration. Updating plugin status, without starting the plugin")
			}

			status := okta.NewPluginOktaStatus(okta.PluginOktaStatusParams{
				SyncSettings: *oktaSpec.GetSyncSettings(),
				ScimEnabled:  scimEnabled,
			})
			okta.ReportPluginStatus(ctx, deps.logger, deps.statusSink, types.PluginStatusCode_RUNNING, status)

			broadcastOktaEvent(deps.parentProcess, plugin, services.OktaReady)
			defer broadcastOktaEvent(deps.parentProcess, plugin, services.OktaStopped)

			<-deps.lifetime.Done()
			return nil
		}, nil
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

			deps.logger.ErrorContext(ctx, "Okta sync is enabled but credentials not found. Updating plugin status, without starting the plugin")

			status := okta.NewPluginOktaStatus(okta.PluginOktaStatusParams{
				SsoConnector: connectorInfo,
				SyncSettings: *oktaSpec.GetSyncSettings(),
				ScimEnabled:  scimEnabled,
				SyncErr:      trace.BadParameter("Okta API credentials not found"),
			})
			okta.ReportPluginStatus(ctx, deps.logger, deps.statusSink, types.PluginStatusCode_OTHER_ERROR, status)

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

func broadcastOktaEvent(process *service.TeleportProcess, plugin *types.PluginV1, eventName string) {
	timestamp := strconv.FormatInt(process.Clock.Now().Unix(), 10)
	pluginName := plugin.GetName()
	process.BroadcastEvent(service.Event{Name: services.EventWithComponents(eventName, pluginName, timestamp), Payload: nil})
}
