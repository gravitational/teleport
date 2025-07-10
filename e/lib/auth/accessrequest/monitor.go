package accessrequest

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/accessmonitoring"
	"github.com/gravitational/teleport/lib/accessmonitoring/review"
	"github.com/gravitational/teleport/lib/backend"
)

const (
	// serviceName specifies the access request monitoring serice name used for debugging.
	serviceName = "access_request_monitoring_service"
)

// Client aggregates the parts of Teleport API client interface
// (as implemented by github.com/gravitational/teleport/api/client.Client)
// that are used by the access plugins.
type Client interface {
	types.Events
	review.Client
}

// Config specifies the access request monitoring service configuration.
type Config struct {
	// Logger is the logger for the access request monitoring serivce.
	Logger *slog.Logger

	// Backend should be a backend.Backend which can be used for obtaining the
	// lock required to run the service.
	Backend backend.Backend

	// Client is the auth service client interface.
	Client Client
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.Backend == nil {
		return trace.BadParameter("backend: must be non-nil")
	}
	if c.Client == nil {
		return trace.BadParameter("client: must be non-nil")
	}
	return nil
}

// MonitoringService monitors access events and applies access monitoring
// rules.
type MonitoringService struct {
	cfg     Config
	monitor *accessmonitoring.AccessMonitor
}

// NewMonitoringSerivce returns a new access request monitoring service.
func NewMonitoringService(cfg Config) (*MonitoringService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "failed to validate access request monitoring service config")
	}

	accessReviewHandler, err := review.NewHandler(review.Config{
		Logger:      cfg.Logger,
		HandlerName: types.BuiltInAutomaticReview,
		Client:      cfg.Client,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	monitor, err := accessmonitoring.NewAccessMonitor(accessmonitoring.Config{
		Logger:  cfg.Logger,
		Backend: cfg.Backend,
		Events:  cfg.Client,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Configure access review handlers.
	monitor.AddAccessMonitoringRuleHandler(accessReviewHandler.HandleAccessMonitoringRule)
	monitor.AddAccessRequestHandler(accessReviewHandler.HandleAccessRequest)

	return &MonitoringService{
		cfg:     cfg,
		monitor: monitor,
	}, nil
}

// Run the access request monitoring service.
func (s *MonitoringService) Run(ctx context.Context) {
	const retryJitter = 10 * time.Second
	const retryInterval = 30 * time.Second
	const lockTTL = time.Minute

	s.cfg.Logger.InfoContext(ctx, "Starting service", "retry_jitter", retryJitter)
	for {
		err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            s.cfg.Backend,
				LockNameComponents: []string{serviceName},
				TTL:                lockTTL,
				RetryInterval:      retryInterval,
			},
		}, s.tryAndCatch)
		if err == nil {
			s.cfg.Logger.InfoContext(ctx, "Exited without error, service will not restart.")
			return
		}
		s.cfg.Logger.ErrorContext(
			ctx,
			"Exited after an error, service will restart after backoff.",
			"error", err,
			"restart_after", retryJitter,
		)
		select {
		case <-ctx.Done():
			s.cfg.Logger.InfoContext(ctx, "Stopping service", "reason", ctx.Err())
			return
		case <-time.After(retryutils.SeventhJitter(retryJitter)):
		}
	}
}

// tryAndCatch tries to run the access request monitoring service and recovers
// from potential panic by converting them into errors. This ensures that a
// critical bug in the service cannot bring down the whole Teleport cluster.
func (s *MonitoringService) tryAndCatch(ctx context.Context) (err error) {
	// If something terribly bad happens while running, we recover and return an error
	defer func() {
		if r := recover(); r != nil {
			s.cfg.Logger.ErrorContext(ctx, "Recovered from panic in service", "panic", r)
			err = trace.NewAggregate(err, trace.Errorf("Panic recovered while running: %v", r))
		}
	}()

	err = trace.Wrap(s.monitor.Run(ctx))
	return
}
