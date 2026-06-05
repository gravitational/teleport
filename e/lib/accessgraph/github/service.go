package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/go-github/v70/github"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

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
)

var (
	// ErrTAGFeatureNotEnabled is returned when the TAG feature is not enabled
	// in the cluster features.
	ErrTAGFeatureNotEnabled = errors.New("TAG feature is not enabled")
	// ErrGithubInvalidCredentials is returned when the Github credentials are
	// invalid.
	ErrGithubInvalidCredentials = errors.New("invalid Github credentials")
)

// Service is a Github service implementation.
type Service struct {
	logger            *slog.Logger
	clock             clockwork.Clock
	accessPoint       types.Semaphores
	accessGraphConfig servicecfg.AccessGraphConfig
	clusterFeatures   func() proto.Features
	hostID            string
	getCreds          accessgraph.ClientCredentialsGetter
	fetcher           *fetcher
	pluginStatusSink  common.StatusSink
	startDate         time.Time
}

// GithubConfig are configuration options for Github.
type GithubConfig struct {
	// Address is the Github URL. If empty, the default Github URL is used.
	Address string
	// PrivateKey is the Github private key used to sign requests.
	PrivateKey []byte
	// ClientID is the Github app client ID.
	// This is used to generate the JWT token.
	ClientID string
	// Organization is the Github organization name.
	// This is used to fetch the audit logs.
	Organization string
	// BootstrapStartDate is the start date for the audit logs.
	BootstrapStartDate time.Time
}

// Config are configuration options for [Service].
type Config struct {
	GithubConfig
	// Clock is the clock used to control time.
	Clock clockwork.Clock
	// Logger is the logger used to log messages.
	Logger *slog.Logger
	// AccessGraphConfig is the configuration for the access graph service.
	AccessGraphConfig servicecfg.AccessGraphConfig
	// HostID is the host ID of the server.
	HostID string
	//  GetCreds is the function used to get the credentials for the access graph service.
	GetCreds accessgraph.ClientCredentialsGetter
	// AccessPoint is the access point used to connect to the auth server.
	AccessPoint types.Semaphores
	// PluginStatusSink is the status sink used to report the status of the plugin.
	PluginStatusSink common.StatusSink
	// ClusterFeatures is the function used to get the cluster features.
	ClusterFeatures func() proto.Features
}

// Validate validates the options.
func (o *Config) Validate() error {
	if len(o.PrivateKey) == 0 {
		return trace.BadParameter("missing Github private key")
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

	return nil
}

// SetDefaults sets the default values for the options.
func (o *Config) SetDefaults() {
	if o.Clock == nil {
		o.Clock = clockwork.NewRealClock()
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.BootstrapStartDate.IsZero() {
		o.BootstrapStartDate = time.Now().Add(-time.Hour * 24)
	}
}

// New creates a new Github service.
func New(ctx context.Context, opts Config) (*Service, error) {
	if err := opts.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	opts.SetDefaults()

	fetcher, err := newFetcher(opts.Logger, opts.Clock, opts.GithubConfig)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create github fetcher")
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
		startDate:         opts.BootstrapStartDate,
	}, nil
}

// Run blocks and runs the Github service.
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
			s.logger.WarnContext(ctx, "Access Graph specified in config, but the license does not include Teleport Identity Security. Access graph sync will not be enabled.")
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
	lockName := func(orgName string) string {
		// github semaphore lock.
		const semaphoreName = "access_graph_github_sync"
		return fmt.Sprintf("%s_%x", semaphoreName, orgName)
	}

	clusterFeatures := s.clusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(ErrTAGFeatureNotEnabled)
	}
	const (
		semaphoreExpiration = time.Minute
	)
	// AcquireSemaphoreLockWithRetry will retry until the semaphore is acquired.
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.accessPoint,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindAccessGraph,
					SemaphoreName: lockName(s.fetcher.organizationName),
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

	const serviceConfig = `{
		"loadBalancingConfig": [{"round_robin": {}}],
		"healthCheckConfig": {
			"serviceName": ""
		}
	}`
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

	// Check if the connection is ready.
	// This will block until the connection is ready or the context is canceled.
	_, err = grpc_health_v1.NewHealthClient(accessGraphConn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return trace.Wrap(err)
	}

	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	go func() {
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.logger.InfoContext(ctx, "access graph service connection was closed")
		}
	}()

	eGroup, ctx := errgroup.WithContext(ctx)

	auditLogErrC := make(chan error, 1)
	eGroup.Go(func() error {
		defer func() {
			close(auditLogErrC)
		}()
		err = s.auditLogsLoop(ctx, client, auditLogErrC)
		return trace.Wrap(err)
	})

	eventStreamErrC := make(chan error, 1)
	eGroup.Go(func() error {
		defer func() {
			close(eventStreamErrC)
		}()
		err = s.githubEventStream(ctx, client, eventStreamErrC)
		return trace.Wrap(err)
	})

	var (
		lastAuditLogErr error
		eventStreamErr  error
		ok              bool
	)
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case lastAuditLogErr, ok = <-auditLogErrC:
			if !ok {
				break loop
			}
		case eventStreamErr, ok = <-eventStreamErrC:
			if !ok {
				break loop
			}
		}
		code := types.PluginStatusCode_RUNNING
		var message string
		var rawErr string

		err = trace.NewAggregate(lastAuditLogErr, eventStreamErr)
		if err != nil {
			code = types.PluginStatusCode_OTHER_ERROR
			message = err.Error()
			rawErr = err.Error()
		}

		if s.pluginStatusSink != nil {
			s.pluginStatusSink.Emit(ctx,
				&types.PluginStatusV1{
					Code:         code,
					LastSyncTime: s.clock.Now(),
					ErrorMessage: message,
					LastRawError: rawErr,
				})
		}
	}
	cancel()
	// Wait for all goroutines to finish.
	return trace.Wrap(eGroup.Wait())
}

// auditLogsLoop polls the Github audit logs and sends them to the access graph service.
// It will retry on errors and back off on rate limit errors.
// It will also send the audit logs to the access graph service in batches.
func (s *Service) auditLogsLoop(ctx context.Context, client accessgraphv1alpha.AccessGraphServiceClient, auditLogErrC chan<- error) error {
	stream, err := client.GitHubAuditLogStream(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to get access graph service stream")
	}
	defer stream.CloseSend()

	err = stream.Send(
		accessgraphv1alpha.GitHubAuditLogStreamRequest_builder{
			Config: accessgraphv1alpha.GitHubConfigV1_builder{
				StartDate:    timestamppb.New(s.startDate),
				Organization: s.fetcher.organizationName,
			}.Build(),
		}.Build(),
	)
	if err != nil {
		err = consumeTillErr(stream)
		return trace.Wrap(err, "failed to send access graph config")
	}

	githubConfig, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, "failed to get github config")
	}

	if githubConfig.GetGithubConfig() == nil {
		return trace.BadParameter("access graph service did not return github config")
	}

	s.logger.InfoContext(ctx, "Access graph service github config", "config", githubConfig.GetGithubConfig())

	resumeState, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, "failed to get github resume state")
	}

	if resumeState.GetAuditLogResumeState() == nil {
		return trace.BadParameter("access graph service did not return github resume state")
	}

	s.logger.InfoContext(ctx, "Access graph service github resume state", "resume_state", resumeState.GetAuditLogResumeState())

	cursor := auditLogCursor{
		token:         resumeState.GetAuditLogResumeState().GetToken(),
		lastID:        resumeState.GetAuditLogResumeState().GetLastEventId(),
		lastTimestamp: resumeState.GetAuditLogResumeState().GetLastEventTime().AsTime(),
	}
	var (
		evts                 []*structpb.Struct
		githubRateLimitError *github.RateLimitError
		isLastPage           bool
	)
	for {
		evts, cursor, isLastPage, err = s.fetcher.pollAuditLogs(ctx,
			githubConfig.GetGithubConfig().GetStartDate().AsTime(),
			cursor,
		)
		if err != nil {
			s.logger.ErrorContext(ctx, "Error polling audit logs", "error", err)
		}

		if len(evts) > 0 {
			sendErr := stream.Send(
				accessgraphv1alpha.GitHubAuditLogStreamRequest_builder{
					AuditLog: accessgraphv1alpha.GitHubAuditLogV1_builder{
						Events: evts,
						Cursor: accessgraphv1alpha.GitHubAuditLogV1Cursor_builder{
							Token:         cursor.token,
							LastEventId:   cursor.lastID,
							LastEventTime: timestamppb.New(cursor.lastTimestamp),
						}.Build(),
					}.Build(),
				}.Build(),
			)
			if sendErr != nil {
				sendErr = consumeTillErr(stream)
				return trace.Wrap(sendErr, "failed to send audit logs")
			}

		}

		select {
		case auditLogErrC <- trace.Wrap(err):
		default:
		}

		waitTime := 10 * time.Second
		switch {
		case errors.As(err, &githubRateLimitError):
			s.logger.DebugContext(ctx, "Github rate limit error", "error", err)
			waitTime = githubRateLimitError.Rate.Reset.Sub(s.clock.Now())
		case isLastPage:
			waitTime = time.Minute
		}

		select {
		case <-ctx.Done():
			return nil
		case <-s.clock.After(waitTime):
		}

	}
}

func (s *Service) githubEventStream(ctx context.Context, client accessgraphv1alpha.AccessGraphServiceClient, errC chan<- error) error {
	stream, err := client.GitHubEventsStream(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to get access graph service stream")
	}
	defer stream.CloseSend()

	_, err = stream.Header()
	if err != nil {
		return trace.Wrap(err, "failed to get access graph service stream header")
	}

	currentTAGResources := &pollResults{}
	timer := s.clock.NewTimer(5 * time.Minute)
	defer timer.Stop()
	for {
		pollResults, err := s.fetcher.pollGithubState(ctx)
		switch {
		case err != nil:
			s.logger.ErrorContext(ctx, "Error reconciling access graph", "error", err)
		default:
			upsert, toDel := reconcileResults(currentTAGResources, pollResults)
			errPush := push(stream, upsert, toDel)
			if errPush != nil {
				return trace.Wrap(errPush, "failed to push resources diff to TAG")
			} else {
				// Update the currentTAGResources with the result of the reconciliation.
				*currentTAGResources = *pollResults
			}
		}

		select {
		case errC <- trace.Wrap(err):
		default:
		}
		if !timer.Stop() {
			select {
			case <-timer.Chan():
			default:
			}
		}
		timer.Reset(5 * time.Minute)

		select {
		case <-ctx.Done():
			return nil
		case <-timer.Chan():
		}

	}
}

const (
	// batchSize is the maximum number of resources to send in a single
	// request to the access graph service.
	batchSize = 500
)

func pushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_GitHubEventsStreamClient,
	upsert *accessgraphv1alpha.GithubResourceList,
) error {
	for send := range slices.Chunk(upsert.GetResources(), batchSize) {
		err := client.Send(
			accessgraphv1alpha.GitHubEventsStreamRequest_builder{
				Upsert: accessgraphv1alpha.GithubResourceList_builder{
					Resources: send,
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
	client accessgraphv1alpha.AccessGraphService_GitHubEventsStreamClient,
	toDel *accessgraphv1alpha.GithubResourceList,
) error {
	for send := range slices.Chunk(toDel.GetResources(), batchSize) {
		err := client.Send(
			accessgraphv1alpha.GitHubEventsStreamRequest_builder{
				Delete: accessgraphv1alpha.GithubResourceList_builder{
					Resources: send,
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
	client accessgraphv1alpha.AccessGraphService_GitHubEventsStreamClient,
	upsert *accessgraphv1alpha.GithubResourceList,
	toDel *accessgraphv1alpha.GithubResourceList,
) (err error) {
	defer func() {
		if err != nil {
			err = consumeTillErr(client)
		}
	}()
	err = pushUpsertInBatches(client, upsert)
	if err != nil {
		return trace.Wrap(err)
	}
	err = pushDeleteInBatches(client, toDel)
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.Send(
		accessgraphv1alpha.GitHubEventsStreamRequest_builder{
			Sync: &accessgraphv1alpha.GithubSync{},
		}.Build(),
	)
	return trace.Wrap(err)
}

func consumeTillErr[T any, K any](stream grpc.BidiStreamingClient[T, K]) error {
	for {
		_, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// GithubInstanceConnectionTest tests the connection to a Github instance.
func GithubInstanceConnectionTest(ctx context.Context, opts GithubConfig) error {
	const (
		applicationNotInstalledMessage = "Application is not installed for the organization %s. " +
			"Please install the application to use Teleport Identity Security."
	)
	fetcher, err := newFetcher(slog.Default(), clockwork.NewRealClock(), opts)
	if errors.Is(err, errApplicationNotInstalled) {
		return trace.BadParameter(applicationNotInstalledMessage, opts.Organization)
	} else if err != nil {
		return trace.Wrap(err, "failed to create github fetcher")
	}

	if err = fetcher.testRequiredPermissions(ctx); errors.Is(err, errApplicationNotInstalled) {
		return trace.BadParameter(applicationNotInstalledMessage, opts.Organization)
	} else if errors.Is(err, ErrGithubInvalidCredentials) {
		const invalidCredsMessage = "Invalid Github credentials. " +
			"Please check your Github app client ID and private key."
		return trace.BadParameter(invalidCredsMessage)
	} else if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
