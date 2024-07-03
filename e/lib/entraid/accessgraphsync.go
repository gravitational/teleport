package entraid

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type AccessGraphConfig struct {
	Clock            clockwork.Clock
	Logger           *slog.Logger
	ConnectionConfig servicecfg.AccessGraphConfig
	Credentials      accessgraph.ClientCredentialsGetter
	SyncSettings     *types.PluginEntraIDAccessGraphSettings
	// GraphClient is the instantiated Microsoft Graph SDK client.
	GraphClient *msgraphsdk.GraphServiceClient

	TenantID string
}

func (cfg *AccessGraphConfig) Validate() error {
	if cfg.Logger == nil {
		return trace.BadParameter("Logger must be specified")
	}
	if cfg.SyncSettings == nil {
		return trace.BadParameter("AccessGraphConfig must be specified")
	}
	if cfg.TenantID == "" {
		return trace.BadParameter("TenantID must be set")
	}
	return nil
}

// SetDefaults sets the default values for the options.
func (cfg *AccessGraphConfig) SetDefaults() {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
}

// AccessGraphSynchronizer synchronizes access graph specific information from Entra ID to TAG.
type AccessGraphSynchronizer struct {
	clock clockwork.Clock
	log   *slog.Logger

	connectionConfig servicecfg.AccessGraphConfig
	credentials      accessgraph.ClientCredentialsGetter
	// appCache is a mapping of AppId to its sso cache entry
	ssoCache map[string]*types.PluginEntraIDAppSSOSettings

	// graphClient is the Microsoft Graph SDKclient
	graphClient graphClient
	// httpClient is the HTTP client used for miscellaneous HTTP operations
	httpClient *http.Client

	tenantID string
}

func NewAccessGraphSynchronizer(cfg AccessGraphConfig) (*AccessGraphSynchronizer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SetDefaults()

	ssoCache := map[string]*types.PluginEntraIDAppSSOSettings{}
	for _, entry := range cfg.SyncSettings.AppSsoSettingsCache {
		ssoCache[entry.AppId] = entry
	}

	return &AccessGraphSynchronizer{
		clock:            cfg.Clock,
		log:              cfg.Logger,
		connectionConfig: cfg.ConnectionConfig,
		credentials:      cfg.Credentials,
		ssoCache:         ssoCache,
		graphClient:      &graphClientWrapper{client: cfg.GraphClient},
		httpClient:       &http.Client{},
		tenantID:         cfg.TenantID,
	}, nil
}

func (s *AccessGraphSynchronizer) Run(ctx context.Context) error {
	const retryDelay = 1 * time.Minute
	for {
		err := s.synchronize(ctx)
		if err != nil {
			s.log.ErrorContext(ctx, "Entra access graph synchronization failed.", "error", err)
			select {
			case <-s.clock.After(retryDelay):
				continue
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		}
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		default:
		}
	}
}

func (s *AccessGraphSynchronizer) synchronize(ctx context.Context) error {
	tagConn, err := newAccessGraphConnection(ctx, s.connectionConfig, s.credentials)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tagConn.Close()

	client := accessgraphv1alpha.NewAccessGraphServiceClient(tagConn)

	stream, err := client.EntraEventsStream(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get access graph service stream", "error", err)
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	go func() {
		defer cancel()
		if !tagConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.log.InfoContext(ctx, "access graph service connection was closed")
		}
	}()

	ticker := s.clock.NewTicker(syncInterval)
	defer ticker.Stop()

	currentTAGResources := &resources{}

	for {
		newResources, err := s.synchronizeOnce(ctx, currentTAGResources, stream)
		if err != nil {
			return trace.Wrap(err)
		}
		currentTAGResources = newResources
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.Chan():
		}
	}
}

// Synchronize syncs objects from Entra to Access Graph once and returns the updated resource cache.
func (s *AccessGraphSynchronizer) synchronizeOnce(ctx context.Context, currentTAGResources *resources, stream accessgraphv1alpha.AccessGraphService_EntraEventsStreamClient) (*resources, error) {
	apps, err := s.fetchApps(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	newResources := &resources{
		Applications: apps,
	}

	upsert, delete := reconcileResults(currentTAGResources, newResources)
	s.log.InfoContext(ctx, "Pushing entra resources into access graph", "upsert_count", len(upsert.Resources), "delete_count", len(delete.Resources))
	err = push(stream, upsert, delete)

	return newResources, trace.Wrap(err)
}

func (s *AccessGraphSynchronizer) fetchApps(ctx context.Context) ([]*accessgraphv1alpha.EntraApplication, error) {
	var entraApps []models.Applicationable
	err := s.graphClient.IterateApplications(ctx, func(app models.Applicationable) bool {
		entraApps = append(entraApps, app)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var results []*accessgraphv1alpha.EntraApplication
	for _, entraApp := range entraApps {
		appID := entraApp.GetAppId()
		if appID == nil {
			s.log.ErrorContext(ctx, "expected app ID to be present")
			continue
		}

		ssoSettings, ok := s.ssoCache[*appID]
		if !ok || ssoSettings == nil {
			s.log.DebugContext(ctx, "entra app does not exist in SSO settings cache, skipping", "app_id", *appID)
			continue
		}

		app, err := s.convertApp(ctx, entraApp, ssoSettings)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to convert entra app to proto", "app_id", *appID)
		}
		results = append(results, app)
	}

	return results, nil
}

func (s *AccessGraphSynchronizer) convertApp(ctx context.Context, app models.Applicationable, ssoSettings *types.PluginEntraIDAppSSOSettings) (*accessgraphv1alpha.EntraApplication, error) {
	appID := app.GetAppId()
	if appID == nil {
		return nil, trace.BadParameter("expected app ID to be present")
	}
	signingCerts, err := getAppSAMLSigningCertificates(ctx, s.httpClient, s.tenantID, *appID)
	if err != nil {
		s.log.InfoContext(ctx, "failed to get signing certificates for entra app")
	}
	protoApp, err := entraAppToProto(ctx, app, ssoSettings, s.tenantID, signingCerts)
	return protoApp, trace.Wrap(err)
}

func newAccessGraphConnection(ctx context.Context, cfg servicecfg.AccessGraphConfig, getCreds accessgraph.ClientCredentialsGetter) (*grpc.ClientConn, error) {
	// Configure health check service to monitor access graph service and
	// automatically reconnect if the connection is lost without
	// relying on new events from the auth server to trigger a reconnect.
	const serviceConfig = `{
		"loadBalancingPolicy": "round_robin",
		"healthCheckConfig": {
			"serviceName": ""
		}
	}`

	tagCfg := accessgraph.ServiceClientConfig{
		Addr:     cfg.Addr,
		Insecure: cfg.Insecure,
		CA:       cfg.CA,
	}

	accessGraphConn, err := accessgraph.NewAccessGraphClient(ctx, tagCfg, getCreds, grpc.WithDefaultServiceConfig(serviceConfig))
	return accessGraphConn, trace.Wrap(err)
}

// resources is a collection of Entra resources
type resources struct {
	Applications []*accessgraphv1alpha.EntraApplication
}

const (
	// batchSize is the maximum number of resources to send in a single
	// request to the access graph service.
	batchSize = 100
)

func pushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_EntraEventsStreamClient,
	toUpsert *accessgraphv1alpha.EntraResourceList,
) error {
	for i := 0; i < len(toUpsert.Resources); i += batchSize {
		end := min(i+batchSize, len(toUpsert.Resources))
		err := client.Send(
			&accessgraphv1alpha.EntraEventsStreamRequest{
				Operation: &accessgraphv1alpha.EntraEventsStreamRequest_Upsert{
					Upsert: &accessgraphv1alpha.EntraResourceList{
						Resources: toUpsert.Resources[i:end],
					},
				},
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func pushDeleteInBatches(
	client accessgraphv1alpha.AccessGraphService_EntraEventsStreamClient,
	toDel *accessgraphv1alpha.EntraResourceList,
) error {
	for i := 0; i < len(toDel.Resources); i += batchSize {
		end := min(i+batchSize, len(toDel.Resources))
		err := client.Send(
			&accessgraphv1alpha.EntraEventsStreamRequest{
				Operation: &accessgraphv1alpha.EntraEventsStreamRequest_Delete{
					Delete: &accessgraphv1alpha.EntraResourceList{
						Resources: toDel.Resources[i:end],
					},
				},
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func push(
	client accessgraphv1alpha.AccessGraphService_EntraEventsStreamClient,
	upsert *accessgraphv1alpha.EntraResourceList,
	toDel *accessgraphv1alpha.EntraResourceList,
) error {
	err := pushUpsertInBatches(client, upsert)
	if err != nil {
		return trace.Wrap(err)
	}
	err = pushDeleteInBatches(client, toDel)
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.Send(
		&accessgraphv1alpha.EntraEventsStreamRequest{
			Operation: &accessgraphv1alpha.EntraEventsStreamRequest_Sync{},
		},
	)
	return trace.Wrap(err)
}
