package auditlogs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/okta/okta-sdk-golang/v2/okta/query"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// ErrTAGFeatureNotEnabled is returned when the TAG feature is not enabled
// in the cluster features.
var ErrTAGFeatureNotEnabled = errors.New("TAG feature is not enabled")

// OktaClient is a stripped down interface for [okta.Client].
type OktaClient interface {
	ListLogEvents(ctx context.Context, qp *query.Params) ([]*okta.LogEvent, *okta.Response, error)
	GetOrgUrl() string
	ListApiTokens(ctx context.Context, qp *query.Params) ([]*oktaapi.ApiToken, *okta.Response, error)
	ListUsersWithRoleAssignments(ctx context.Context) (*oktaapi.RoleAssignedUsers, *okta.Response, error)
	ListAssignedRolesForUser(ctx context.Context, userId string) ([]*okta.Role, *okta.Response, error)
	GetAuthorizedScopes(ctx context.Context) ([]string, error)
}

// Service is a Okta AuditLogs service implementation.
type Service struct {
	client            OktaClient
	logger            *slog.Logger
	clock             clockwork.Clock
	accessPoint       types.Semaphores
	accessGraphConfig servicecfg.AccessGraphConfig
	clusterFeatures   func() proto.Features
	hostID            string
	getCreds          accessgraph.ClientCredentialsGetter
	startDate         time.Time
	orgURL            string
	reportStatus      ReportStatusFunc
}

// ReportStatusFunc is a function that reports the status of the Okta AuditLogs service.
type ReportStatusFunc func(ctx context.Context, now time.Time, err error)

// Config are configuration options for [Service].
type Config struct {
	// Client is the Okta client to use.
	Client OktaClient
	// Clock is the clock to use for scheduling.
	Clock clockwork.Clock
	// Logger is the logger to use.
	Logger *slog.Logger
	// AccessGraphConfig is the configuration for the access graph service.
	AccessGraphConfig servicecfg.AccessGraphConfig
	// HostID is the ID of the host.
	HostID string
	// GetCreds is the function to get the credentials for the access graph service.
	GetCreds accessgraph.ClientCredentialsGetter
	// AccessPoint is the access point to use.
	AccessPoint types.Semaphores
	// ClusterFeatures is the function to get the cluster features.
	ClusterFeatures func() proto.Features
	// BootstrapStartDate is the date to start syncing from.
	BootstrapStartDate time.Time
	// ReportStatus is the reporter to use for reporting status updates.
	ReportStatus ReportStatusFunc
}

// Validate validates the options.
func (o *Config) Validate() error {
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

	if o.ReportStatus == nil {
		return trace.BadParameter("missing report status function")
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
		o.BootstrapStartDate = time.Now().Add(-time.Hour * 24 * 10)
	}
}

// New creates a new Okta AuditLogs service.
func New(ctx context.Context, opts Config) (*Service, error) {
	if err := opts.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	opts.SetDefaults()

	u, err := url.Parse(opts.Client.GetOrgUrl())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse org URL")
	}

	if err := oktacommon.CheckClientOAuthScopes(ctx,
		opts.Client,
		oktacommon.GetSIEMOAuthScopes()...,
	); err != nil {
		return nil, trace.BadParameter(
			"Unable to export Okta System Logs due to missing permissions. "+
				"Your Okta API credentials lack the required %v scopes needed for this operation. "+
				"Please review your Okta API Service permissions to ensure you have the necessary scopes, and try again.",
			oktacommon.GetSIEMOAuthScopes(),
		)
	}

	return &Service{
		client:            opts.Client,
		startDate:         opts.BootstrapStartDate,
		logger:            opts.Logger,
		clock:             opts.Clock,
		accessPoint:       opts.AccessPoint,
		accessGraphConfig: opts.AccessGraphConfig,
		clusterFeatures:   opts.ClusterFeatures,
		hostID:            opts.HostID,
		getCreds:          opts.GetCreds,
		orgURL:            u.Host,
		reportStatus:      opts.ReportStatus,
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
			s.logger.ErrorContext(ctx, "Error initializing and watching access graph", "error", err)
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
	clusterFeatures := s.clusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(ErrTAGFeatureNotEnabled)
	}

	const (
		semaphoreNamePrefix = "access_graph_okta_audit_logs_sync"
		semaphoreExpiration = time.Minute
	)
	semaphoreName := fmt.Sprintf("%s-%x", semaphoreNamePrefix, s.orgURL)
	// AcquireSemaphoreLockWithRetry will retry until the semaphore is acquired.
	// This prevents multiple Okta Audit Log services to push System logs in parallel.
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

	const (
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

	eGroup.Go(func() (err error) {
		defer cancel()
		err = s.exportAuditLogs(ctx, client)
		s.reportStatus(ctx, s.clock.Now(), err)
		return trace.Wrap(err)
	})

	eGroup.Go(func() (err error) {
		defer cancel()
		err = s.exportEventStream(ctx, client)
		return trace.Wrap(err)
	})

	// Wait for all goroutines to finish.
	return trace.Wrap(eGroup.Wait())
}

func (s *Service) exportAuditLogs(ctx context.Context, client accessgraphv1alpha.AccessGraphServiceClient) error {
	stream, err := client.OktaAuditLogStream(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stream.CloseSend()

	// Teleport always sends the bootStrap start date to the access graph service.
	// This is used to determine the start date for the audit logs if there is no
	// resume state available in the access graph service. If there is a resume
	// state available, the access graph service will send that state to the
	// Okta Audit Log service.
	err = stream.Send(
		&accessgraphv1alpha.OktaAuditLogStreamRequest{
			Operation: &accessgraphv1alpha.OktaAuditLogStreamRequest_Config{
				Config: &accessgraphv1alpha.OktaConfigV1{
					StartDate:    timestamppb.New(s.startDate),
					Organization: s.orgURL,
				},
			},
		},
	)
	if err != nil {
		err = consumeTillErr(stream)
		return trace.Wrap(err, "failed to send access graph config")
	}

	// Receive the access graph config and resume state from the access graph service.
	// This value can be different from the config value sent above if the access
	// graph service has a resume state available. The access graph service will
	// send the resume state to the Okta Audit Log service.
	// If there is no resume state available, the access graph service will
	// send the start date value sent above.
	tagOktaConfig, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, "failed to get access graph config")
	}

	if tagOktaConfig.GetConfig() == nil {
		return trace.BadParameter("access graph service did not return okta config")
	}

	s.logger.InfoContext(ctx, "Access graph service okta config", "config", tagOktaConfig.GetConfig())

	resumeState, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, "failed to get okta resume state")
	}

	if resumeState.GetAuditLogResumeState() == nil {
		return trace.BadParameter("access graph service did not return github resume state")
	}
	s.logger.InfoContext(ctx, "Access graph service github resume state", "resume_state", resumeState.GetAuditLogResumeState())

	cursor := cursor{
		after:         resumeState.GetAuditLogResumeState().GetToken(),
		lastEventID:   resumeState.GetAuditLogResumeState().GetLastEventId(),
		lastEventTime: resumeState.GetAuditLogResumeState().GetLastEventTime().AsTime(),
	}
	var (
		evts []*accessgraphv1alpha.OktaEventV1
	)
	for {
		evts, cursor, err = s.pollAuditLogs(ctx,
			tagOktaConfig.GetConfig().GetStartDate().AsTime(),
			cursor,
		)
		if err != nil {
			s.logger.ErrorContext(ctx, "Error polling audit logs", "error", err)
		}

		s.reportStatus(ctx, s.clock.Now(), err)

		if len(evts) > 0 {
			sendErr := stream.Send(
				&accessgraphv1alpha.OktaAuditLogStreamRequest{
					Operation: &accessgraphv1alpha.OktaAuditLogStreamRequest_AuditLog{
						AuditLog: &accessgraphv1alpha.OktaAuditLogV1{
							Events: evts,
							Cursor: &accessgraphv1alpha.OktaAuditLogV1Cursor{
								Token:         cursor.after,
								LastEventId:   cursor.lastEventID,
								LastEventTime: timestamppb.New(cursor.lastEventTime),
							},
						},
					},
				},
			)
			if sendErr != nil {
				sendErr = consumeTillErr(stream)
				return trace.Wrap(sendErr, "failed to send audit logs")
			}

		}

		waitTime := 10 * time.Second
		if err == nil {
			waitTime = time.Minute
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.clock.After(waitTime):
		}

	}
}

func (s *Service) exportEventStream(ctx context.Context, client accessgraphv1alpha.AccessGraphServiceClient) error {
	stream, err := client.OktaEventsStream(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to get access graph service stream")
	}
	defer stream.CloseSend()

	// Access Graph service always sends a gRPC header with the supported
	// resource types. Currently we don't use it, but we consume it to
	// ensure Access Graph is behaving correctly. This value will be
	// used in the future to determine which resources are supported by
	// the Access Graph service version.
	_, err = stream.Header()
	if err != nil {
		return trace.Wrap(err, "failed to get access graph service stream header")
	}

	currentTAGResources := &pollResults{}
	for {
		pollResults, err := s.fetchOktaState(ctx)
		var oktaErr *okta.Error
		switch {
		case errors.As(err, &oktaErr) && oktaErr.ErrorCode == oktaapi.OktaErrCodeAccessDeniedException:
			s.logger.InfoContext(ctx, "Okta integration misses Super Administrator Permissions. "+
				"Okta API tokens and roles will not be exported to the access graph service.")
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
		case <-ctx.Done():
			return nil
		case <-s.clock.After(30 * time.Minute):
		}

	}
}

const (
	// batchSize is the maximum number of resources to send in a single
	// request to the access graph service.
	batchSize = 500
)

func pushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_OktaEventsStreamClient,
	upsert *accessgraphv1alpha.OktaResourceList,
) error {
	for send := range slices.Chunk(upsert.Resources, batchSize) {
		err := client.Send(
			&accessgraphv1alpha.OktaEventsStreamRequest{
				Operation: &accessgraphv1alpha.OktaEventsStreamRequest_Upsert{
					Upsert: &accessgraphv1alpha.OktaResourceList{
						Resources: send,
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
	client accessgraphv1alpha.AccessGraphService_OktaEventsStreamClient,
	toDel *accessgraphv1alpha.OktaResourceList,
) error {
	for send := range slices.Chunk(toDel.Resources, batchSize) {
		err := client.Send(
			&accessgraphv1alpha.OktaEventsStreamRequest{
				Operation: &accessgraphv1alpha.OktaEventsStreamRequest_Delete{
					Delete: &accessgraphv1alpha.OktaResourceList{
						Resources: send,
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
	client accessgraphv1alpha.AccessGraphService_OktaEventsStreamClient,
	upsert *accessgraphv1alpha.OktaResourceList,
	toDel *accessgraphv1alpha.OktaResourceList,
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
		&accessgraphv1alpha.OktaEventsStreamRequest{
			Operation: &accessgraphv1alpha.OktaEventsStreamRequest_Sync{
				Sync: &accessgraphv1alpha.OktaSync{},
			},
		},
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
