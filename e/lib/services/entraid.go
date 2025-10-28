package services

import (
	"context"
	"errors"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/entraid"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/msgraph"
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

func startEntraIDService(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, spec *types.PluginEntraIDSettings, integrationSpec *types.AzureOIDCIntegrationSpecV1) error {
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

	var credential msgraph.AzureTokenProvider
	// Construct MS Graph Client
	if usesSystemCredentials(spec) {
		credential, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return trace.Wrap(err, "failed to create Azure default credential")
		}
	} else if integrationSpec != nil {
		getAssertion := func(ctx context.Context) (string, error) {
			token, err := azureoidc.GenerateEntraOIDCToken(ctx, authServer, authServer.GetKeyStore(), process.Clock)
			if err != nil {
				return "", trace.Wrap(err)
			}
			return token, nil
		}
		credential, err = azidentity.NewClientAssertionCredential(integrationSpec.TenantID, integrationSpec.ClientID, getAssertion, nil)
		if err != nil {
			return trace.Wrap(err, "failed to create Azure client assertion credential")
		}
	} else {
		return trace.BadParameter("Azure OIDC integration spec is required for Entra ID service when system credentials are not used")
	}

	graphClient, err := constructGraphClient(credential, process.MetricsRegistry())
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
		Clock:           process.Clock,
		Logger:          logger.With(teleport.ComponentKey, teleport.Component(eteleport.ComponentEntraIDDirectoryReconciler, process.GetID())),
		MetricsRegistry: process.MetricsRegistry(),
		GraphClient:     graphClient,
		UserSvc:         authServer,
		AccessListSvc:   authServer,
		SAMLSvc:         authServer,
		DefaultOwners:   owners,
		TenantID:        tenantID,
		EntraAppID:      appID,
		SSOConnectorID:  spec.SyncSettings.SsoConnectorId,
		GroupsFilter:    groupsFilters,
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
func EntraIDPluginInit(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, spec *types.PluginEntraIDSettings, integrationSpec *types.AzureOIDCIntegrationSpecV1) (string, error) {
	if process == nil {
		return "", trace.BadParameter("process required")
	}

	// Set the expected instance role for this identity event since it's unique to this plugin.
	process.SetExpectedInstanceRole(types.RoleAccessGraphPlugin, entraIDIdentityEvent)

	process.RegisterFunc("entraid.init", func() error {
		return startEntraIDService(ctx, process, statusSink, spec, integrationSpec)
	})

	return EventWithComponents(EntraIDStoppedEvent), nil
}

// constructGraphClient returns a new MS Graph API client using the given function to retrieve the client assertion.
func constructGraphClient(credential msgraph.AzureTokenProvider, registerer prometheus.Registerer) (*msgraph.Client, error) {
	if registerer != nil {
		registerer = prometheus.WrapRegistererWith(prometheus.Labels{teleport.ComponentLabel: eteleport.ComponentEntraID}, registerer)
	}

	graphClient, err := msgraph.NewClient(msgraph.Config{
		TokenProvider:   credential,
		MetricsRegistry: registerer,
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
