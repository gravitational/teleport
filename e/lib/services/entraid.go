package services

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/entraid"
	"github.com/gravitational/teleport/e/lib/entraid/accessgraph"
	"github.com/gravitational/teleport/e/lib/entraid/directory"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/plugins/filter"
	"github.com/gravitational/teleport/lib/service"
)

const (
	entraIDIdentityEvent = "EntraIDIdentity"
	// EntraIDReadyEvent is generated when the EntraID service is started.
	EntraIDReadyEvent = "EntraIDReady"
	// EntraIDStoppedEvent is generated when the EntraID service is stopped.
	EntraIDStoppedEvent = "EntraIDStopped"
)

func startEntraIDService(ctx context.Context, reg *metrics.Registry, process *service.TeleportProcess, statusSink common.StatusSink, spec *types.PluginEntraIDSettings, integrationSpec *types.AzureOIDCIntegrationSpecV1) error {
	logger := process.Config.Logger.With(teleport.ComponentKey, teleport.Component(eteleport.ComponentEntraID, process.GetID()))
	features := process.Config.Modules.Features()
	if !features.GetEntitlement(entitlements.Identity).Enabled {
		logger.ErrorContext(ctx, "Entra ID service requires Teleport Identity Governance. Entra ID sync will not run.")
		return nil
	}

	// Register our request for AccessGraphPlugin credentials.
	process.RegisterWithAuthServer(types.RoleAccessGraphPlugin, entraIDIdentityEvent)

	authServer := process.GetAuthServer()

	// Wait for EntraID credentials.
	conn, err := process.WaitForConnector(entraIDIdentityEvent, logger)
	if err != nil {
		return trace.Wrap(err)
	}

	if conn == nil {
		// Is the server shutting down? Report back.
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter("failed to acquire AccessGraphPlugin credentials from Auth")
	}

	graphClient, err := constructGraphClient(spec, integrationSpec, process, reg.Wrap("msgraph"))
	if err != nil {
		return trace.Wrap(err)
	}

	// Construct directory reconciler

	owners := []accesslist.Owner{}
	for _, name := range spec.SyncSettings.DefaultOwners {
		owners = append(owners, accesslist.Owner{Name: name})
	}

	tenantID, err := getTenantID(spec, integrationSpec)
	if err != nil {
		return trace.Wrap(err, "failed to get tenant ID")
	}

	appID, err := getAppID(ctx, spec, authServer)
	if err != nil {
		return trace.Wrap(err, "failed to get app ID")
	}

	groupsFilters, err := filter.New(spec.SyncSettings.GroupFilters)
	if err != nil {
		// unknown filter type is handled within the
		// directory reconciler service.
		if !errors.Is(err, filter.ErrUnknownFilter) {
			if statusSink != nil {
				statusSink.Emit(ctx, &types.PluginStatusV1{
					Code:         types.PluginStatusCode_OTHER_ERROR,
					ErrorMessage: "Failed to initialize group filters",
					LastRawError: err.Error(),
				})
			}
			return trace.Wrap(err)
		}
	}

	syncIntervals := entraSyncIntervals(ctx, spec.SyncSettings.SyncIntervals, logger)

	directoryReconciler, err := directory.New(directory.Config{
		Clock:                  process.Clock,
		Logger:                 logger.With(teleport.ComponentKey, teleport.Component(eteleport.ComponentEntraIDDirectoryReconciler, process.GetID())),
		MetricsRegistry:        reg.Wrap("directory"),
		GraphClient:            graphClient,
		AccessPoint:            authServer,
		DefaultOwners:          owners,
		TenantID:               tenantID,
		EntraAppID:             appID,
		SSOConnectorID:         spec.SyncSettings.SsoConnectorId,
		GroupsFilter:           groupsFilters,
		AccessListOwnersSource: spec.SyncSettings.AccessListOwnersSource,
		DeltaSyncEnabled:       syncIntervals.Delta > 0,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Construct Access Graph reconciler. This remains nil if access graph sync is not enabled.
	var tagSynchronizer *accessgraph.Synchronizer
	if spec.AccessGraphSettings != nil && features.AccessGraph {
		tagCfg := process.Config.AccessGraph
		if !tagCfg.Enabled || tagCfg.Addr == "" {
			return trace.BadParameter("Access graph synchronization requested, but access graph is not configured ")
		}

		tagSynchronizer, err = accessgraph.NewSynchronizer(accessgraph.Config{
			Logger:           logger,
			ConnectionConfig: tagCfg,
			Credentials:      conn.ClientGetCertificate,
			SyncSettings:     spec.AccessGraphSettings,
			GraphClient:      graphClient,
			TenantID:         tenantID,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Construct the main Entra ID service

	svc, err := entraid.New(entraid.Config{
		Logger:                  logger,
		PluginStatusSink:        statusSink,
		DirectoryReconciler:     directoryReconciler,
		AccessGraphSynchronizer: tagSynchronizer,
		SemaphoreSvc:            authServer,
		HostID:                  conn.HostUUID(),
		Clock:                   process.Clock,
		SyncIntervals:           &syncIntervals,
	})
	if err != nil {
		if statusSink != nil {
			statusSink.Emit(ctx, &types.PluginStatusV1{
				Code:         types.PluginStatusCode_OTHER_ERROR,
				LastRawError: err.Error(),
			})
		}
		return trace.Wrap(err)
	}

	// Update plugin status if the service is running as a plugin.
	if statusSink != nil {
		statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING})
	}

	// Broadcast that we are ready and start.
	process.BroadcastEvent(service.Event{Name: EntraIDReadyEvent})
	err = svc.Run(ctx)

	process.BroadcastEvent(service.Event{Name: EntraIDStoppedEvent})
	return trace.Wrap(err)
}

// EntraIDPluginInit initializes hosted Entra ID service (hosted plugin).
// Returns immediately.
func EntraIDPluginInit(ctx context.Context, reg *metrics.Registry, process *service.TeleportProcess, statusSink common.StatusSink, spec *types.PluginEntraIDSettings, integrationSpec *types.AzureOIDCIntegrationSpecV1) (string, error) {
	if process == nil {
		return "", trace.BadParameter("process required")
	}

	// Set the expected instance role for this identity event since it's unique to this plugin.
	process.SetExpectedInstanceRole(types.RoleAccessGraphPlugin, entraIDIdentityEvent)

	process.RegisterFunc("entraid.init", func() error {
		return startEntraIDService(ctx, reg, process, statusSink, spec, integrationSpec)
	})

	return EventWithComponents(EntraIDStoppedEvent), nil
}

// constructGraphClient returns a new MS Graph API client using the given function to retrieve the client assertion.
func constructGraphClient(pluginSpec *types.PluginEntraIDSettings, integrationSpec *types.AzureOIDCIntegrationSpecV1, process *service.TeleportProcess, reg *metrics.Registry) (*msgraph.Client, error) {
	if reg != nil {
		reg = reg.Wrap("msclient")
	}

	var httpClient *http.Client
	if process.Config.Testing.HTTPTransport != nil {
		clt, err := defaults.HTTPClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clt.Transport = process.Config.Testing.HTTPTransport
		httpClient = clt
	}

	credential, err := makeCredentialProvider(pluginSpec, integrationSpec, process, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	graphClient, err := msgraph.NewClient(msgraph.Config{
		TokenProvider:   credential,
		MetricsRegistry: reg,
		HTTPClient:      httpClient,
	})

	return graphClient, trace.Wrap(err)
}

func usesSystemCredentials(plugin *types.PluginEntraIDSettings) bool {
	return plugin.SyncSettings != nil && plugin.SyncSettings.CredentialsSource == types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS
}

func getTenantID(spec *types.PluginEntraIDSettings, integrationSpec *types.AzureOIDCIntegrationSpecV1) (string, error) {
	tenantID := spec.SyncSettings.TenantId
	if tenantID == "" && integrationSpec != nil {
		// backfill tenant ID from integration spec
		tenantID = integrationSpec.TenantID
	} else if tenantID == "" && integrationSpec == nil {
		return "", trace.BadParameter("Tenant ID is required for Entra ID service")
	}
	return tenantID, nil
}

func getAppID(ctx context.Context, spec *types.PluginEntraIDSettings, authServer interface {
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
}) (string, error) {
	appID := spec.SyncSettings.EntraAppId
	if appID != "" {
		return appID, nil
	}

	connector, err := authServer.GetSAMLConnector(ctx, spec.SyncSettings.SsoConnectorId, false)
	if err != nil {
		return "", trace.Wrap(err, "failed to get SSO connector")
	}

	u, err := url.Parse(connector.GetEntityDescriptorURL())
	if err != nil {
		return "", trace.Wrap(err, "failed to parse entity descriptor URL")
	}

	appID = u.Query().Get("appid")

	return appID, nil
}

func makeCredentialProvider(
	pluginSpec *types.PluginEntraIDSettings,
	integrationSpec *types.AzureOIDCIntegrationSpecV1,
	process *service.TeleportProcess,
	client *http.Client,
) (msgraph.AzureTokenProvider, error) {
	if usesSystemCredentials(pluginSpec) {
		opts := &azidentity.DefaultAzureCredentialOptions{}
		if client != nil {
			// client only expected in tests.
			opts.ClientOptions = azcore.ClientOptions{
				Transport: client,
			}
		}
		credential, err := azidentity.NewDefaultAzureCredential(opts)
		if err != nil {
			return nil, trace.Wrap(err, "failed to create Azure default credential")
		}
		return credential, nil
	}

	if integrationSpec == nil {
		return nil, trace.BadParameter("Azure OIDC integration spec is required for Entra ID service when system credentials are not used")
	}

	authServer := process.GetAuthServer()
	getAssertion := func(ctx context.Context) (string, error) {
		token, err := azureoidc.GenerateEntraOIDCToken(ctx, authServer, authServer.GetKeyStore(), process.Clock)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return token, nil
	}
	credential, err := azidentity.NewClientAssertionCredential(integrationSpec.TenantID, integrationSpec.ClientID, getAssertion, nil)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create Azure client assertion credential")
	}

	return credential, nil
}

func entraSyncIntervals(ctx context.Context, in *types.PluginEntraIDSyncIntervals, logger *slog.Logger) entraid.SyncIntervals {
	if in == nil {
		return entraid.SyncIntervals{Full: entraid.DefaultFullSyncInterval}
	}
	// Empty full and delta interval is expected in existing plugin installation.
	if in.Delta == "" && in.Full == "" {
		return entraid.SyncIntervals{Full: entraid.DefaultFullSyncInterval}
	}

	syncIntervals := entraid.SyncIntervals{
		Delta: parseIntervals(ctx, in.Delta, "delta", logger),
		Full:  parseIntervals(ctx, in.Full, "full", logger),
	}

	if syncIntervals.Full == 0 && syncIntervals.Delta == 0 {
		// Note: full=0 and delta=0 config is allowed to be able
		// to support "pause" behavior, which is not implemented yet.
		// 5 minutes remains the default backward compatibility.
		logger.WarnContext(ctx, `Entra ID service requires at least one full or delta sync interval. `+
			`Service will fallback to full sync running at 5 minutes interval.`)
		syncIntervals.Full = entraid.DefaultFullSyncInterval
	}

	if syncIntervals.Delta > 0 &&
		syncIntervals.Full > 0 &&
		syncIntervals.Delta >= syncIntervals.Full {

		logger.WarnContext(ctx, `Delta sync interval is equal or greater than full sync, `+
			`delta sync will be skipped.`)
		syncIntervals.Delta = 0
	}

	return syncIntervals
}

func parseIntervals(ctx context.Context, interval string, syncMode string, logger *slog.Logger) time.Duration {
	if interval == "" {
		return 0
	}

	d, err := time.ParseDuration(interval)
	if err != nil {
		logger.ErrorContext(ctx,
			`Failed to parse sync interval`,
			"sync_mode", syncMode,
			"error", err,
			"interval", interval)
		return 0
	}

	return d
}
