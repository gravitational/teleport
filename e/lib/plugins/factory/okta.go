package factory

import (
	"context"
	"strconv"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	oktausermonitor "github.com/gravitational/teleport/e/lib/okta/usermonitor"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
)

// Okta will create Okta services based on the plugin specification.
func Okta(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	oktaSpec := plugin.Spec.GetOkta()
	if oktaSpec == nil {
		return nil, trace.BadParameter("field Spec.Okta must be present")
	}

	syncEnabled := oktaSpec.GetSyncSettings().GetEnableUserSync()

	_, scimEnabled, err := oktaplugin.SelectSCIMTokenHash(deps.StaticCredentials)
	if err != nil {
		return nil, trace.Wrap(err, "checking if SCIM support is enabled")
	}

	var connectorInfo types.SAMLConnector
	ssoConnectorId := oktaSpec.GetSyncSettings().SsoConnectorId

	if ssoConnectorId != "" {
		authServer := deps.ParentProcess.GetAuthServer()
		connectorInfo, err = authServer.GetSAMLConnector(ctx, ssoConnectorId, false)
		if err != nil {
			return nil, trace.Wrap(err, "fetching auth connector")
		}
	}

	// If sync is not enabled (SSO-only/SCIM-only integration) then only report the plugin's
	// status and wait for the context to be canceled.
	if !syncEnabled {
		return func(ctx context.Context) error {
			if scimEnabled {
				deps.Logger.InfoContext(ctx, "SCIM-only integration. Updating plugin status, without starting the plugin")
			} else {
				deps.Logger.InfoContext(ctx, "SSO-only integration. Updating plugin status, without starting the plugin")
			}

			status := okta.NewPluginOktaStatus(okta.PluginOktaStatusParams{
				SsoConnector: connectorInfo,
				SyncSettings: *oktaSpec.GetSyncSettings(),
				ScimEnabled:  scimEnabled,
			})
			okta.ReportPluginStatus(ctx, deps.Logger, deps.StatusSink, types.PluginStatusCode_RUNNING, status)

			broadcastOktaEvent(deps.ParentProcess, plugin, services.OktaReady)
			defer broadcastOktaEvent(deps.ParentProcess, plugin, services.OktaStopped)

			<-ctx.Done()
			return nil
		}, nil
	}

	var oktaAuthProvider oktaapi.AuthProvider
	selectedOktaCreds, err := oktaplugin.SelectOktaCredentials(deps.StaticCredentials)
	if err != nil {
		return nil, trace.Wrap(err, "looking up for Okta credentials")
	}
	switch {
	case selectedOktaCreds.OauthClientId != "":
		oktaAuthProvider = oktaapi.NewOauthProviderWithOktaCASigner(ctx, oktaapi.OauthOktaCACredentialsConfig{
			OAuthClientID: selectedOktaCreds.OauthClientId,
			AuthService:   deps.ParentProcess.GetAuthServer().Cache,
			CAKeyStore:    deps.ParentProcess.GetAuthServer().GetKeyStore(),
			Clock:         deps.ParentProcess.Clock,
		})
	case selectedOktaCreds.ApiToken != "":
		oktaAuthProvider = oktaapi.NewSSWSAuthProvider(selectedOktaCreds.ApiToken)
	default:
		return func(ctx context.Context) error {
			deps.Logger.ErrorContext(ctx, "Okta sync is enabled but credentials not found. Updating plugin status, without starting the plugin")

			status := okta.NewPluginOktaStatus(okta.PluginOktaStatusParams{
				SsoConnector: connectorInfo,
				SyncSettings: *oktaSpec.GetSyncSettings(),
				ScimEnabled:  scimEnabled,
				SyncErr:      trace.BadParameter("Okta API credentials not found"),
			})
			okta.ReportPluginStatusError(ctx, deps.Logger, deps.StatusSink, types.PluginStatusCode_OKTA_CONFIG_ERROR, status, "Sync is enabled, but Okta credentials are missing.")

			<-ctx.Done()
			return nil
		}, nil
	}

	// TODO: Propagate license changes to Okta hosted plugin runtime.
	// Currently, if license gets upgraded, okta service will still be
	// running with stale settings (unless it was restarted).
	oktaSpec.SyncSettings.SyncUsers = oktaSpec.SyncSettings.SyncUsers && modules.GetModules().Features().GetEntitlement(entitlements.OktaUserSync).Enabled
	oktaSpec.SyncSettings.DisableSyncAppGroups = oktaSpec.SyncSettings.DisableSyncAppGroups || selectedOktaCreds.ApiTokenForSCIMOnly
	return func(ctx context.Context) error {
		// Bind Okta Plugin monitor lifecycle to the plugin runtime.
		// When the plugin runtime stops, the monitor will also be stopped
		// via the context cancellation. the Plugin Monitor manages the lifecycle.
		// and if plugin monitor detects plugin removed it will also cancel the context.
		//
		// The oktausermonitor is responsible for monitoring user RBAC changes
		// and translating them into Okta Assignments that. That allows to
		// have async mechanism for syncing user changed RBAC checks made in
		// Teleport to Okta without requiring a user to re-login.
		userMonitor, err := oktausermonitor.New(oktausermonitor.Config{
			Logger:            deps.Logger,
			Clock:             deps.ParentProcess.Clock,
			AuthServer:        deps.ParentProcess.GetAuthServer(),
			Events:            deps.ParentProcess.GetAuthServer(),
			Backend:           deps.ParentProcess.GetBackend(),
			ReconcileInterval: deps.ParentProcess.Config.UserMonitor.ReconcileInterval,
			LockTTL:           deps.ParentProcess.Config.UserMonitor.LockTTL,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		userMonitor.Start(ctx)

		closeEvent := services.InitOktaPlugin(ctx,
			services.OktaPluginPrams{
				Process:          deps.ParentProcess,
				PluginStatusSink: deps.StatusSink,
				PluginName:       plugin.GetName(),
				OrgUrl:           oktaSpec.OrgUrl,
				AuthProvider:     oktaAuthProvider,
				SyncSettings:     *oktaSpec.SyncSettings,
				SCIMEnabled:      scimEnabled,
				Plugin:           plugin,
			},
		)
		// wait for the calling context to finish before doing anything else.
		<-ctx.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err = deps.ParentProcess.WaitForEvent(eventCtx, closeEvent); err != nil {
			deps.Logger.DebugContext(ctx, "Error waiting for OktaStopped event", "error", err)
			return trace.Wrap(err)
		}
		deps.Logger.InfoContext(ctx, "Okta plugin has stopped")
		return nil
	}, nil
}

func broadcastOktaEvent(process *service.TeleportProcess, plugin *types.PluginV1, eventName string) {
	timestamp := strconv.FormatInt(process.Clock.Now().Unix(), 10)
	pluginName := plugin.GetName()
	process.BroadcastEvent(service.Event{Name: services.EventWithComponents(eventName, pluginName, timestamp), Payload: nil})
}
