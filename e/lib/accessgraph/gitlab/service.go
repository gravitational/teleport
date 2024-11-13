package gitlab

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

var (
	// ErrTAGFeatureNotEnabled is returned when the TAG feature is not enabled
	// in the cluster features.
	ErrTAGFeatureNotEnabled = errors.New("TAG feature is not enabled")
	// ErrGitlabInvalidCredentials is returned when the Gitlab credentials are
	// invalid.
	ErrGitlabInvalidCredentials = errors.New("invalid Gitlab credentials")
)

// Service is a Gitlab service implementation.
type Service struct {
	logger            *slog.Logger
	clock             clockwork.Clock
	accessPoint       types.Semaphores
	accessGraphConfig servicecfg.AccessGraphConfig
	clusterFeatures   func() proto.Features
	hostID            string
	getCreds          accessgraph.ClientCredentialsGetter
	fetcher           *gitlabFetcher
	pluginStatusSink  common.StatusSink
	usageReporter     usagereporter.UsageReporter
}

// GitlabOpts are configuration options for Gitlab.
type GitlabOpts struct {
	// Address is the Gitlab URL.
	Address string
	// Token is the Gitlab API token.
	Token string
}

// Opts are configuration options for [Service].
type Opts struct {
	GitlabOpts
	Clock             clockwork.Clock
	Logger            *slog.Logger
	AccessGraphConfig servicecfg.AccessGraphConfig
	HostID            string
	GetCreds          accessgraph.ClientCredentialsGetter
	AccessPoint       types.Semaphores
	PluginStatusSink  common.StatusSink
	ClusterFeatures   func() proto.Features
	UsageReporter     usagereporter.UsageReporter
}

// Validate validates the options.
func (o *Opts) Validate() error {
	if o.Address == "" {
		return trace.BadParameter("missing Gitlab address")
	}
	if o.Token == "" {
		return trace.BadParameter("missing Gitlab token")
	}

	if o.AccessGraphConfig.Addr == "" {
		return trace.BadParameter("missing access graph address")
	}

	if o.HostID == "" {
		return trace.BadParameter("missing host ID")
	}

	if o.AccessPoint == nil {
		return trace.BadParameter("missing access point")
	}
	if o.ClusterFeatures == nil {
		return trace.BadParameter("missing cluster features")
	}

	if o.UsageReporter == nil {
		return trace.BadParameter("missing usage reporter")
	}

	return nil
}

// SetDefaults sets the default values for the options.
func (o *Opts) SetDefaults() {
	if o.Clock == nil {
		o.Clock = clockwork.NewRealClock()
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
}

// New creates a new Gitlab service.
func New(ctx context.Context, opts Opts) (*Service, error) {
	if err := opts.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	opts.SetDefaults()

	fetcher, err := newGitlabFetcher(opts.Address, opts.Token)
	if err != nil {
		return nil, trace.Wrap(&pollError{err: err})
	}
	return &Service{
		logger:            opts.Logger,
		clock:             opts.Clock,
		accessPoint:       opts.AccessPoint,
		accessGraphConfig: opts.AccessGraphConfig,
		clusterFeatures:   opts.ClusterFeatures,
		hostID:            opts.HostID,
		getCreds:          opts.GetCreds,
		fetcher:           fetcher,
		pluginStatusSink:  opts.PluginStatusSink,
		usageReporter:     opts.UsageReporter,
	}, nil
}

// Run blocks and runs the Gitlab service.
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

	for {
		// reset the currentTAGResources to force a full sync
		if err := s.initializeAndWatchAccessGraph(ctx); errors.Is(err, ErrTAGFeatureNotEnabled) {
			s.logger.WarnContext(ctx, "Access Graph specified in config, but the license does not include Teleport Policy. Access graph sync will not be enabled.")
			break
		} else if err != nil {
			s.logger.WarnContext(ctx, "Error initializing and watching access graph", "error", err)
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
		// gitlab semaphore lock.
		semaphoreName = "access_graph_gitlab_sync"
		// Configure health check service to monitor access graph service and
		// automatically reconnect if the connection is lost without
		// relying on new events from the auth server to trigger a reconnect.
		serviceConfig = `{
		 "loadBalancingPolicy": "round_robin",
		 "healthCheckConfig": {
			 "serviceName": ""
		 }
	 }`
	)

	clusterFeatures := s.clusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(ErrTAGFeatureNotEnabled)
	}
	const (
		semaphoreExpiration = time.Minute
	)
	// AcquireSemaphoreLockWithRetry will retry until the semaphore is acquired.
	// This prevents multiple discovery services to push AWS resources in parallel.
	// lease must be released to cleanup the resource in auth server.
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.accessPoint,
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

	stream, err := client.GitlabEventsStream(ctx)
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
	ticker := s.clock.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		err = s.reconcileAccessGraph(ctx, currentTAGResources, stream)
		code := types.PluginStatusCode_RUNNING
		message := GitlabHumanReadableError(err)
		rawErr := GitlabRawError(err)
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
					Details: &types.PluginStatusV1_Gitlab{
						Gitlab: &types.PluginGitlabStatusV1{
							ImportedUsers:    uint32(len(currentTAGResources.Users)),
							ImportedGroups:   uint32(len(currentTAGResources.Groups)),
							ImportedProjects: uint32(len(currentTAGResources.Projects)),
						},
					},
				})
		}

		s.usageReporter.AnonymizeAndSubmit(
			&usagereporter.AccessGraphGitlabScanEvent{
				TotalUsers:    uint64(len(currentTAGResources.Users)),
				TotalGroups:   uint64(len(currentTAGResources.Groups)),
				TotalProjects: uint64(len(currentTAGResources.Projects)),
			},
		)

		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.Chan():
		}
	}
}

func (s *Service) reconcileAccessGraph(ctx context.Context, currentTAGResources *resources, stream accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient) error {
	result, errPoll := s.fetcher.poll(ctx)
	if errPoll != nil {
		s.logger.ErrorContext(ctx, "Error polling gitlab resources", "error", errPoll)
		errPoll = &pollError{err: errPoll}
	}

	// if result is nil, it means Teleport couldn't retrieve the resources
	// from Gitlab due to a connection problem or misconfiguration.
	// If an error exists, we don't reconcile.
	if result == nil {
		return trace.Wrap(errPoll)
	}

	upsert, toDel := reconcileResults(currentTAGResources, result)
	errPush := push(stream, upsert, toDel)
	if errPush != nil {
		s.logger.ErrorContext(ctx, "Error pushing resources diff to TAG", "error", errPush)
		errPush = &accessGraphPushError{err: errPush}
		return trace.NewAggregate(errPoll, errPush)
	}
	// Update the currentTAGResources with the result of the reconciliation.
	*currentTAGResources = *result
	return trace.NewAggregate(errPoll, errPush)
}

const (
	// batchSize is the maximum number of resources to send in a single
	// request to the access graph service.
	batchSize = 500
)

func pushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient,
	upsert *accessgraphv1alpha.GitlabResourceList,
) error {
	for i := 0; i < len(upsert.Resources); i += batchSize {
		end := i + batchSize
		if end > len(upsert.Resources) {
			end = len(upsert.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.GitlabEventsStreamRequest{
				Operation: &accessgraphv1alpha.GitlabEventsStreamRequest_Upsert{
					Upsert: &accessgraphv1alpha.GitlabResourceList{
						Resources: upsert.Resources[i:end],
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
	client accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient,
	toDel *accessgraphv1alpha.GitlabResourceList,
) error {
	for i := 0; i < len(toDel.Resources); i += batchSize {
		end := i + batchSize
		if end > len(toDel.Resources) {
			end = len(toDel.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.GitlabEventsStreamRequest{
				Operation: &accessgraphv1alpha.GitlabEventsStreamRequest_Delete{
					Delete: &accessgraphv1alpha.GitlabResourceList{
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
	client accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient,
	upsert *accessgraphv1alpha.GitlabResourceList,
	toDel *accessgraphv1alpha.GitlabResourceList,
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
		&accessgraphv1alpha.GitlabEventsStreamRequest{
			Operation: &accessgraphv1alpha.GitlabEventsStreamRequest_Sync{},
		},
	)
	return trace.Wrap(err)
}
