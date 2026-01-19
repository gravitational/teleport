package services

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/entraid"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/modules"
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
	features := modules.GetModules().Features()
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

	directoryReconciler, err := entraid.NewDirectoryReconciler(entraid.DirectoryReconcilerConfig{
		Clock:                  process.Clock,
		Logger:                 logger.With(teleport.ComponentKey, teleport.Component(eteleport.ComponentEntraIDDirectoryReconciler, process.GetID())),
		MetricsRegistry:        reg.Wrap("directory"),
		GraphClient:            graphClient,
		UserSvc:                authServer,
		AccessListSvc:          authServer,
		SAMLSvc:                authServer,
		DefaultOwners:          owners,
		TenantID:               tenantID,
		EntraAppID:             appID,
		SSOConnectorID:         spec.SyncSettings.SsoConnectorId,
		GroupsFilter:           groupsFilters,
		AccessListOwnersSource: spec.SyncSettings.AccessListOwnersSource,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Construct Access Graph reconciler. This remains nil if access graph sync is not enabled.
	var tagSynchronizer *entraid.AccessGraphSynchronizer
	if spec.AccessGraphSettings != nil && features.AccessGraph {
		tagCfg := process.Config.AccessGraph
		if !tagCfg.Enabled || tagCfg.Addr == "" {
			return trace.BadParameter("Access graph synchronization requested, but access graph is not configured ")
		}

		tagSynchronizer, err = entraid.NewAccessGraphSynchronizer(entraid.AccessGraphConfig{
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

	svc, err := entraid.NewService(entraid.ServiceConfig{
		Logger:                  logger,
		PluginStatusSink:        statusSink,
		DirectoryReconciler:     directoryReconciler,
		AccessGraphSynchronizer: tagSynchronizer,
		SemaphoreSvc:            authServer,
		HostID:                  conn.HostUUID(),
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
