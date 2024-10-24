package secreports

import (
	"context"
	"log/slog"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/utils/retryutils"
	cloudaws "github.com/gravitational/teleport/e/lib/cloud/aws"
	"github.com/gravitational/teleport/e/lib/secreports/limiter"
	"github.com/gravitational/teleport/e/lib/secreports/query"
	"github.com/gravitational/teleport/e/lib/secreports/query/athena"
	"github.com/gravitational/teleport/e/lib/secreports/reports"
	"github.com/gravitational/teleport/e/lib/secreports/scheduler"
	"github.com/gravitational/teleport/e/lib/secreports/store"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// queryProvider is the query provider interface.
type queryProvider interface {
	// RunQuery runs the query.
	RunQuery(ctx context.Context, query string, days int) (*query.RunQueryResponse, error)
	// GetQueryResult gets the query result.
	GetQueryResult(ctx context.Context, queryID, nextToken string, maxResults int32) (*query.GetQueryResultResponse, error)
}

type reportResultStore interface {
	// SaveReportResult saves the report result.
	SaveReportResult(ctx context.Context, name string, result *pb.ReportResult) error
	// LoadReportResult loads the report result.
	LoadReportResult(ctx context.Context, name string) (*pb.ReportResult, error)
}

// ServiceConfig is the service config for the Access Lists gRPC service.
type ServiceConfig struct {
	// Limiter is the cost limiter.
	Limiter *limiter.Limiter
	// AthenaURL is audit events the Athena URL.
	AthenaURL string
	// Logger is the logger to use.
	Logger *slog.Logger
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer
	// Clock is the clock.
	Clock clockwork.Clock
	// Storage is a backed storage object.
	Storage services.SecReports
	// Semaphore is the semaphore to use.
	Semaphore types.Semaphores
	// ProcessContext is the process context.
	ProcessContext context.Context
	// Emitter is audit event emitter.
	Emitter apievents.Emitter
	// Backend is the backend to use.
	Backend backend.Backend
	// AccessMonitoring is the access monitoring configuration.
	AccessMonitoring *servicecfg.AccessMonitoringOptions
	// LimiterStorage is the cost limiter storage.
	LimiterStorage services.CostLimiter
	// Region is the AWS region.
	Region string
	// MaxParallelUserQueries is the query limiter that limits the number of async queries.
	// This is a soft limit to prevent overloading the Athena service by running user queries and draining the
	// ParallelQueryExecutions Athena limit. Note that this limit is per auth server.
	MaxParallelUserQueries int
}

const (
	// defaultMaxParallelUserQueries is the maximum number of parallel user queries.
	// This is a soft limit to prevent overloading the Athena service by running user queries and draining the
	// ParallelQueryExecutions Athena limit.
	// https://docs.aws.amazon.com/athena/latest/ug/service-limits.html
	defaultMaxParallelUserQueries = 3
)

// CheckAndSetDefaults validates the config and sets default values.
func (c *ServiceConfig) CheckAndSetDefaults() error {
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer param is missing")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "secreports")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Storage == nil {
		return trace.BadParameter("storage param is missing")
	}
	if c.ProcessContext == nil {
		return trace.BadParameter("process context is missing")
	}
	if c.Emitter == nil {
		return trace.BadParameter("emitter is missing")
	}
	if c.ProcessContext == nil {
		return trace.BadParameter("process context is missing")
	}
	if c.LimiterStorage == nil {
		return trace.BadParameter("failed to initialize cost limiter storage")
	}
	if c.AccessMonitoring == nil {
		return trace.BadParameter("access monitoring config is missing")
	}
	if c.Limiter == nil {
		return trace.BadParameter("limiter is missing")
	}
	if c.MaxParallelUserQueries == 0 {
		c.MaxParallelUserQueries = defaultMaxParallelUserQueries
	}
	return nil
}

func buildAthenaConfig(cfg ServiceConfig) (*athena.Config, error) {
	athenaConfig, err := athena.ConfigFromURI(cfg.AthenaURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.AccessMonitoring.ReportResults != "" {
		athenaConfig.ReportResults = cfg.AccessMonitoring.ReportResults
	}
	if len(athenaConfig.RoleARN) == 0 {
		athenaConfig.RoleARN = cfg.AccessMonitoring.RoleARN
	}
	if athenaConfig.Database == "" {
		athenaConfig.Database = cfg.AccessMonitoring.Database
	}
	if athenaConfig.Table == "" {
		athenaConfig.Table = cfg.AccessMonitoring.Table
	}
	if athenaConfig.Workgroup == "" {
		athenaConfig.Workgroup = cfg.AccessMonitoring.Workgroup
	}
	if athenaConfig.QueryResults == "" {
		athenaConfig.QueryResults = cfg.AccessMonitoring.QueryResults
	}
	athenaConfig.Clock = cfg.Clock
	return athenaConfig, nil
}

// NewService creates a new Access List gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	ctx := cfg.ProcessContext
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	awsConfig, err := cloudaws.BuildAWSConfig(ctx, cfg.Region, cfg.AccessMonitoring.RoleARN, cfg.AccessMonitoring.RoleTags)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	athenaConfig, err := buildAthenaConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	athena, err := athena.NewAthena(athenaConfig, awsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sched, err := scheduler.New(scheduler.Config{
		Limiter:     cfg.Limiter,
		Clock:       cfg.Clock,
		MinInterval: time.Hour,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s3store, err := store.NewS3(store.S3Config{
		AWSConfig: awsConfig,
		S3URI:     athenaConfig.ReportResults,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:                cfg.Logger,
		authorizer:         cfg.Authorizer,
		clock:              cfg.Clock,
		storage:            cfg.Storage,
		limiterStorage:     cfg.LimiterStorage,
		athena:             &queryLimiter{queryProvider: athena, limiter: cfg.Limiter},
		semaphore:          cfg.Semaphore,
		emitter:            cfg.Emitter,
		backend:            cfg.Backend,
		reportStore:        s3store,
		ParentCtx:          cfg.ProcessContext,
		Scheduler:          sched,
		userQueriesLimiter: limiter.NewUserQuery(defaultMaxParallelUserQueries),
	}, nil
}

// Service implements gRPC SecReportsServiceServer Methods.
type Service struct {
	log                *slog.Logger
	authorizer         authz.Authorizer
	semaphore          types.Semaphores
	clock              clockwork.Clock
	storage            services.SecReports
	limiterStorage     services.CostLimiter
	athena             queryProvider
	reportStore        reportResultStore
	emitter            apievents.Emitter
	backend            backend.Backend
	ParentCtx          context.Context
	Scheduler          *scheduler.Scheduler
	userQueriesLimiter *limiter.UserQuery

	pb.UnimplementedSecReportsServiceServer
}

// Init initializes the service.
func (s *Service) Init(ctx context.Context) error {
	if err := s.initPrebuiltReports(ctx); err != nil {
		return trace.Wrap(err)
	}
	go s.runPredictablyOnSingleAuth(ctx, s.schedulesReportsUpdate)
	return nil
}

func (s *Service) initPrebuiltReports(ctx context.Context) error {
	err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            s.backend,
			LockNameComponents: []string{"security_report_init_prebuilt_lock"},
			TTL:                time.Second * 30,
			RetryInterval:      time.Millisecond * 200,
		},
	}, func(ctx context.Context) error {
		for _, report := range reports.PrebuiltReports {
			if err := s.maybeUpdateReport(ctx, report); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	return trace.Wrap(err)
}

// reportValidDaysRange is a valid days range for the report time rage.
// Right now we support only 7, 30, 90, 120 days range.
var reportValidDaysRange = []int32{7, 30, 90, 120}

// getReportExecutionDaysRange returns a valid days range for the report time rage.
// If access monitoring is enabled, the function returns a range up to max report range
// where unsupported days are filtered out.
func getReportExecutionDaysRange() []int32 {
	f := modules.GetModules().Features()
	entitlement := f.GetEntitlement(entitlements.AccessMonitoring)
	if entitlement.Enabled && entitlement.Limit == 0 {
		return reportValidDaysRange
	}
	var out []int32
	for _, v := range reportValidDaysRange {
		if v > entitlement.Limit {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (s *Service) maybeUpdateSecurityReports(ctx context.Context, threshold time.Duration) error {
	reports, err := s.storage.GetSecurityReports(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, report := range reports {
		for _, days := range getReportExecutionDaysRange() {
			if err := s.runReport(ctx, report, days, withReportReadyRerunThreshold(threshold)); err != nil {
				s.log.ErrorContext(ctx, "Failed to run report", "name", report.GetName(), "days", days, "error", err)
			}
		}
	}
	return nil
}

func (s *Service) maybeUpdateReport(ctx context.Context, report *reports.AuditReportType) error {
	oldReport, err := s.storage.GetSecurityReport(ctx, report.Name)
	switch {
	case err == nil:
		if !reportUpdated(oldReport.Spec.Version, report.Version) {
			return nil
		}
	case trace.IsNotFound(err):
		// If report is not found, create it.
	case err != nil:
		return trace.Wrap(err)
	}

	rep, err := reports.ToSecurityReportType(report)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.storage.UpsertSecurityReport(ctx, rep); err != nil {
		return trace.Wrap(err)
	}
	s.log.InfoContext(ctx, "Report version updated", slog.Group("report", "name", report.Name, "version", report.Version))
	return nil
}

func reportUpdated(oldVersion, newVersion string) bool {
	oldSemV, err := semver.NewVersion(oldVersion)
	if err != nil {
		return true
	}
	newSemV, err := semver.NewVersion(newVersion)
	if err != nil {
		return true
	}
	if !oldSemV.LessThan(*newSemV) {
		return false
	}
	return true
}

func (s *Service) updateReportState(ctx context.Context, reportName string, status secreports.Status) error {
	state, err := secreports.NewReportState(header.Metadata{Name: reportName}, secreports.ReportStateSpec{
		Status:    status,
		UpdatedAt: s.clock.Now().UTC(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err = s.storage.UpsertSecurityReportsState(ctx, state); err != nil {
		return trace.Wrap(err)
	}
	s.log.DebugContext(ctx, "Report execution state updated", "report_name", reportName, "state", status)
	return nil
}

// acquireRunningPhase acquires report running state making sure that only one report execution is running at a time.
// If reports is running or was recently executing according to reportReadyThreshold
// the function returns if execution should be performed.
func (s *Service) acquireRunningPhase(ctx context.Context, executionName string, triggerThreshold time.Duration) (bool, error) {
	lease, err := services.AcquireSemaphoreWithRetry(ctx, services.AcquireSemaphoreWithRetryConfig{
		Service: s.semaphore,
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.KindSecurityReportState,
			SemaphoreName: executionName,
			MaxLeases:     1,
			Expires:       s.clock.Now().Add(time.Minute),
		},
		Retry: retryutils.LinearConfig{
			Step:  time.Second,
			Max:   time.Second,
			Clock: s.clock,
		},
	})
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer func() {
		err := s.semaphore.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			s.log.ErrorContext(ctx, "Failed to cancel lease", slog.Group("lease", "name", lease.SemaphoreName, "id", lease.LeaseID), slog.Any("error", err))
		}
	}()

	shouldRun, err := s.shouldRunReport(ctx, executionName, triggerThreshold)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if !shouldRun {
		return false, nil
	}
	if err = s.updateReportState(ctx, executionName, secreports.Running); err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}

type runReportOptions struct {
	// triggerThreshold limits how often report can be re-run.
	// If report was successfully executed less than triggerThreshold ago it will not be re-run.
	triggerThreshold time.Duration
}
type runReportOptionFunc func(*runReportOptions)

func withReportReadyRerunThreshold(threshold time.Duration) runReportOptionFunc {
	return func(o *runReportOptions) {
		o.triggerThreshold = threshold
	}
}

const (
	// defaultReportReadyRerunUserThreshold limits how often report can be re-run by user request.
	defaultReportReadyRerunUserThreshold = time.Minute
)

func (s *Service) runReport(ctx context.Context, report *secreports.Report, days int32, options ...runReportOptionFunc) error {
	opts := runReportOptions{
		triggerThreshold: defaultReportReadyRerunUserThreshold,
	}
	for _, o := range options {
		o(&opts)
	}

	executionName := secreports.ReportExecutionName(report.GetName(), days)

	ok, err := s.acquireRunningPhase(ctx, executionName, opts.triggerThreshold)
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		return nil
	}
	if _, err := s.runReportAndUpdateState(ctx, report, days); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) runPredictablyOnSingleAuth(ctx context.Context, call func(context.Context) error) {
	const (
		waitTime = time.Hour
		lockName = "access_monitoring_scheduler_auth_lock"
	)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.log.DebugContext(ctx, "Acquiring auth lock for reports scheduler")
			err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
				LockConfiguration: backend.LockConfiguration{
					Backend:            s.backend,
					LockNameComponents: []string{lockName},
					TTL:                time.Hour,
					RetryInterval:      time.Minute,
				},
			}, func(ctx context.Context) error {
				if err := call(ctx); err != nil {
					return trace.Wrap(err)
				}
				return nil
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				s.log.WarnContext(ctx, "Could not get lock", "error", err)
			}
		}
		select {
		case <-time.After(waitTime):
			continue
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) schedulesReportsUpdate(ctx context.Context) error {
	t, err := s.Scheduler.Next(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	now := s.clock.Now()
	if now.After(t) || now.Equal(t) {
		// Check if report state is stale depending on def defaultReportReadyRerunSchedulerThreshold.
		// If report was executed by a user and is still running the function will not re-run it.
		if err := s.maybeUpdateSecurityReports(ctx, defaultReportUpdateThreshold); err != nil {
			s.log.ErrorContext(ctx, "Failed to update security reports", "error", err)
		}
	}
	return nil
}

const (
	defaultReportUpdateThreshold = time.Hour * 24
)

type queryLimiter struct {
	queryProvider
	limiter *limiter.Limiter
}

func (q *queryLimiter) RunQuery(ctx context.Context, query string, days int) (*query.RunQueryResponse, error) {
	releaseFn, err := q.limiter.AllocateLimit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := q.queryProvider.RunQuery(ctx, query, days)
	if err != nil {
		return nil, trace.Wrap(err, releaseFn(0))
	}
	return resp, trace.Wrap(releaseFn(uint64(resp.DataScannedInBytes)))
}
