package resourceusagev1

import (
	context "context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/resourceusage"
)

// ServiceConfig contains parameters and dependencies for the resource usage Service.
type ServiceConfig struct {
	AuditLog            events.AuditLogger
	Authorizer          authz.Authorizer
	Clock               clockwork.Clock
	GetDevicesUsageFunc func(ctx context.Context, f *modules.Features) (*resourceusagepb.DevicesUsage, error)
}

// checkAndSetDefaults checks and sets the defaults.
func (c *ServiceConfig) checkAndSetDefaults() error {
	switch {
	case c.AuditLog == nil:
		return trace.BadParameter("param AuditLog must be specified")
	case c.Authorizer == nil:
		return trace.BadParameter("param Authorizer must be specified")
	case c.GetDevicesUsageFunc == nil:
		return trace.BadParameter("param GetDevicesUsageFunc must be specified")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// Service implements resource usage gRPC server.
type Service struct {
	resourceusagepb.UnimplementedResourceUsageServiceServer

	auditLog            events.AuditLogger
	authorizer          authz.Authorizer
	clock               clockwork.Clock
	getDevicesUsageFunc func(ctx context.Context, f *modules.Features) (*resourceusagepb.DevicesUsage, error)
}

// New creates a new Service according to the config.
func New(cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		auditLog:            cfg.AuditLog,
		authorizer:          cfg.Authorizer,
		clock:               cfg.Clock,
		getDevicesUsageFunc: cfg.GetDevicesUsageFunc,
	}, nil
}

// GetUsage implements resourceusagev1.ResourceUsageServiceServer.
func (s *Service) GetUsage(ctx context.Context, in *resourceusagepb.GetUsageRequest) (*resourceusagepb.GetUsageResponse, error) {
	if _, err := s.authorizer.Authorize(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	f := modules.GetModules().Features()
	if !f.IsUsageBasedBilling {
		return &resourceusagepb.GetUsageResponse{
			AccountUsageType: resourceusagepb.AccountUsageType_ACCOUNT_USAGE_TYPE_UNLIMITED,
			AccessRequests:   &resourceusagepb.AccessRequestsUsage{},
			DevicesUsage:     &resourceusagepb.DevicesUsage{},
		}, nil // unlimited
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(4) // arbitrary

	var accessRequests *resourceusagepb.AccessRequestsUsage
	g.Go(func() error {
		var err error
		accessRequests, err = s.getAccessRequestsUsage(gCtx, &f)
		return trace.Wrap(err)
	})

	var devicesUsage *resourceusagepb.DevicesUsage
	g.Go(func() error {
		var err error
		devicesUsage, err = s.getDevicesUsageFunc(gCtx, &f)
		return trace.Wrap(err)
	})

	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &resourceusagepb.GetUsageResponse{
		AccountUsageType: resourceusagepb.AccountUsageType_ACCOUNT_USAGE_TYPE_USAGE_BASED,
		AccessRequests:   accessRequests,
		DevicesUsage:     devicesUsage,
	}, nil
}

func (s *Service) getAccessRequestsUsage(ctx context.Context, f *modules.Features) (*resourceusagepb.AccessRequestsUsage, error) {
	monthlyUsed, err := resourceusage.GetAccessRequestMonthlyUsage(ctx, s.auditLog, s.clock.Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &resourceusagepb.AccessRequestsUsage{
		MonthlyLimit: int32(f.AccessRequests.MonthlyRequestLimit),
		MonthlyUsed:  int32(monthlyUsed),
	}, nil
}
