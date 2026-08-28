package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	netiqclient "github.com/gravitational/teleport/e/lib/netiq/client"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

var (
	// ErrTAGFeatureNotEnabled is returned when the TAG feature is not enabled
	// in the cluster features.
	ErrTAGFeatureNotEnabled = errors.New("TAG feature is not enabled")
	// ErrNetIQInvalidCredentials is returned when the NetIQ credentials are
	// invalid.
	ErrNetIQInvalidCredentials = errors.New("invalid NetIQ credentials")
)

// Service is the NetIQ service.
type Service struct {
	client            *netiqclient.Client
	clock             clockwork.Clock
	pluginStatusSink  common.StatusSink
	semaphoreSvc      types.Semaphores
	hostID            string
	logger            *slog.Logger
	accessGraphConfig servicecfg.AccessGraphConfig
	getCreds          accessgraph.ClientCredentialsGetter
	clusterFeatures   func() proto.Features
}

// Config is the configuration for the NetIQ service.
type Config struct {
	// Clock is the clock used by the service.
	Clock clockwork.Clock
	// PluginStatusSink is the sink used to report plugin status.
	PluginStatusSink common.StatusSink
	// SemaphoreSvc is the semaphore service used by the service.
	SemaphoreSvc types.Semaphores
	// HostID is the ID of the host running the service.
	HostID string
	// Logger is the logger used by the service.
	Logger *slog.Logger
	// ClientConfig is the configuration for the NetIQ client.
	ClientConfig ClientConfig
	// AccessGraphConfig is the configuration for the Access Graph.
	AccessGraphConfig servicecfg.AccessGraphConfig
	// GetCreds is the function used to get client credentials.
	GetCreds accessgraph.ClientCredentialsGetter
	// GetClusterFeatures is the function used to get cluster features.
	ClusterFeatures func() proto.Features
}

// ClientConfig is the configuration for the NetIQ client.
type ClientConfig struct {
	// OAuthClientID is the OAuth Client ID used to authenticate to OSP(NetIQ authorization service).
	OAuthClientID string
	// OAuthClientSecret is the OAuth Client Secret used to authenticate to OSP(NetIQ authorization service).
	OAuthClientSecret string
	// OSPURL is the URL of the OSP(NetIQ authorization service).
	OSPURL string
	// APIURL is the URL of the IDMProv API.
	APIURL string
	// IdentityVaultUser is the user used to authenticate to the Identity Vault.
	IdentityVaultUser string
	// IdentityVaultPassword is the password used to authenticate to the Identity Vault.
	IdentityVaultPassword string
	// InsecureSkipVerify is a flag that determines whether to skip verification of the server's certificate chain and host name.
	InsecureSkipVerify bool
}

// New creates a new NetIQ service using the given configuration.
func New(ctx context.Context, cfg Config) (*Service, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, trace.Wrap(err, "failed to validate config")
	}

	// Create the NetIQ client.
	client, err := netiqclient.New(
		ctx,
		netiqclient.Config{
			OAuthClientID:         cfg.ClientConfig.OAuthClientID,
			OAuthClientSecret:     cfg.ClientConfig.OAuthClientSecret,
			OSPURL:                cfg.ClientConfig.OSPURL,
			APIURL:                cfg.ClientConfig.APIURL,
			IdentityVaultUser:     cfg.ClientConfig.IdentityVaultUser,
			IdentityVaultPassword: cfg.ClientConfig.IdentityVaultPassword,
			InsecureSkipVerify:    cfg.ClientConfig.InsecureSkipVerify,
			Clock:                 cfg.Clock,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create NetIQ client")
	}

	return &Service{
		client:            client,
		clock:             cfg.Clock,
		pluginStatusSink:  cfg.PluginStatusSink,
		semaphoreSvc:      cfg.SemaphoreSvc,
		hostID:            cfg.HostID,
		logger:            cfg.Logger,
		accessGraphConfig: cfg.AccessGraphConfig,
		getCreds:          cfg.GetCreds,
		clusterFeatures:   cfg.ClusterFeatures,
	}, nil
}

// Run blocks and runs the NetIQ service.
func (s *Service) Run(ctx context.Context) error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  defaults.HighResPollingPeriod,
		Driver: retryutils.NewExponentialDriver(defaults.HighResPollingPeriod),
		Max:    defaults.LowResPollingPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  s.clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	defer s.client.Close()

	for {
		// reset the currentTAGResources to force a full sync
		if err := s.initializeAndWatchAccessGraph(ctx); errors.Is(err, ErrTAGFeatureNotEnabled) {
			s.logger.WarnContext(ctx, "Access Graph specified in config, but the license does not include Teleport Identity Security. Access Graph sync will not be enabled.")
			break
		} else if err != nil {
			s.logger.WarnContext(ctx, "Error initializing and watching Access Graph", "error", err)
		}
		retry.Inc()
		select {
		case <-ctx.Done():
			return nil
		case <-s.clock.After(retry.Duration()):
		}
	}
	return nil
}

// initializeAndWatchAccessGraph creates a new access graph service client and
// watches the connection state. If the connection is closed, it will
// automatically try to reconnect.
func (s *Service) initializeAndWatchAccessGraph(ctx context.Context) error {
	const (
		// netiq semaphore lock.
		semaphoreName = "access_graph_netiq_sync"
		// Configure health check service to monitor access graph service and
		// automatically reconnect if the connection is lost without
		// relying on new events from the auth server to trigger a reconnect.
		serviceConfig = `{
		 "loadBalancingConfig": [{"round_robin": {}}],
		 "healthCheckConfig": {
			 "serviceName": ""
		 }
	 }`
	)

	clusterFeatures := s.clusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.AccessGraph)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(ErrTAGFeatureNotEnabled)
	}
	const (
		semaphoreExpiration = time.Minute
	)
	// AcquireSemaphoreLockWithRetry will retry until the semaphore is acquired.
	// This prevents multiple discovery services to push NetIQ resources in parallel.
	// lease must be released to cleanup the resource in auth server.
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.semaphoreSvc,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindAccessGraph,
					SemaphoreName: semaphoreName,
					MaxLeases:     1,
					Holder:        s.hostID,
				},
				Expiry: semaphoreExpiration,
				Clock:  s.clock,
			},
			Retry: retryutils.LinearConfig{
				Clock:  s.clock,
				First:  time.Second,
				Step:   semaphoreExpiration / 2,
				Max:    semaphoreExpiration,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(lease)
	defer cancel()
	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.logger.WarnContext(ctx, "error cleaning up semaphore", "error", err)
		}
	}()

	accessGraphConn, err := accessgraph.NewAccessGraphClient(
		ctx,
		accessgraph.ServiceClientConfig{
			Addr:     s.accessGraphConfig.Addr,
			CA:       s.accessGraphConfig.CA,
			Insecure: s.accessGraphConfig.Insecure,
		},
		s.getCreds,
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Close the connection when the function returns.
	defer accessGraphConn.Close()
	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	stream, err := client.NetIQEventsStream(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get access graph service stream", "error", err)
		return trace.Wrap(err)
	}

	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	go func() {
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.logger.InfoContext(ctx, "access graph service connection was closed")
		}
	}()

	currentTAGResources := &resources{}
	const interval = 5 * time.Minute
	ticker := s.clock.NewTimer(interval)
	defer ticker.Stop()
	for {
		err = s.reconcileAccessGraph(ctx, currentTAGResources, stream)
		code := types.PluginStatusCode_RUNNING
		message := NetIQHumanReadableError(err)
		rawErr := NetIQRawError(err)
		if err != nil {
			code = types.PluginStatusCode_OTHER_ERROR
			if isUnauthorized(err) {
				code = types.PluginStatusCode_UNAUTHORIZED
			}
		}

		if s.pluginStatusSink != nil {
			s.pluginStatusSink.Emit(ctx,
				&types.PluginStatusV1{
					Code:         code,
					LastSyncTime: s.clock.Now(),
					ErrorMessage: message,
					LastRawError: rawErr,
					Details: &types.PluginStatusV1_NetIq{
						NetIq: &types.PluginNetIQStatusV1{
							ImportedUsers:     uint32(len(currentTAGResources.users)),
							ImportedGroups:    uint32(len(currentTAGResources.groups)),
							ImportedResources: uint32(len(currentTAGResources.resources)),
							ImportedRoles:     uint32(len(currentTAGResources.roles)),
						},
					},
				})
		}

		ticker.Reset(interval)
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.Chan():
		}
	}
}

func (s *Service) reconcileAccessGraph(ctx context.Context, currentTAGResources *resources, stream accessgraphv1alpha.AccessGraphService_NetIQEventsStreamClient) error {
	result, errPoll := s.pullNetIQData(ctx)
	if errPoll != nil {
		s.logger.ErrorContext(ctx, "Error polling netiq resources", "error", errPoll)
		errPoll = &pollError{err: errPoll}
		return trace.Wrap(errPoll)
	}

	upsert, toDel := reconcileResults(currentTAGResources, &result)
	errPush := push(stream, upsert, toDel)
	if errPush != nil {
		s.logger.ErrorContext(ctx, "Error pushing resources diff to TAG", "error", errPush)
		errPush = &accessGraphPushError{err: errPush}
		return trace.NewAggregate(errPoll, errPush)
	}
	// Update the currentTAGResources with the result of the reconciliation.
	*currentTAGResources = result
	return trace.NewAggregate(errPoll, errPush)
}

const (
	// batchSize is the maximum number of resources to send in a single
	// request to the access graph service.
	batchSize = 500
)

func pushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_NetIQEventsStreamClient,
	upsert *accessgraphv1alpha.NetIQResourceList,
) error {
	for resources := range slices.Chunk(upsert.GetResources(), batchSize) {
		err := client.Send(
			accessgraphv1alpha.NetIQEventsStreamRequest_builder{
				Upsert: accessgraphv1alpha.NetIQResourceList_builder{
					Resources: resources,
				}.Build(),
			}.Build(),
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func pushDeleteInBatches(
	client accessgraphv1alpha.AccessGraphService_NetIQEventsStreamClient,
	toDel *accessgraphv1alpha.NetIQResourceList,
) error {
	for resources := range slices.Chunk(toDel.GetResources(), batchSize) {
		err := client.Send(
			accessgraphv1alpha.NetIQEventsStreamRequest_builder{
				Delete: accessgraphv1alpha.NetIQResourceList_builder{
					Resources: resources,
				}.Build(),
			}.Build(),
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func push(
	client accessgraphv1alpha.AccessGraphService_NetIQEventsStreamClient,
	upsert *accessgraphv1alpha.NetIQResourceList,
	toDel *accessgraphv1alpha.NetIQResourceList,
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
		accessgraphv1alpha.NetIQEventsStreamRequest_builder{
			Sync: &accessgraphv1alpha.NetIQSyncOperation{},
		}.Build(),
	)
	return trace.Wrap(err)
}
