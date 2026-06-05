package resourceusagev1

import (
	context "context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/resourceusage"
)

// ServiceConfig contains parameters and dependencies for the resource usage Service.
type ServiceConfig struct {
	AuditLog   events.AuditLogger
	Modules    modules.Modules
	Authorizer authz.Authorizer
	Clock      clockwork.Clock
}

// checkAndSetDefaults checks and sets the defaults.
func (c *ServiceConfig) checkAndSetDefaults() error {
	switch {
	case c.AuditLog == nil:
		return trace.BadParameter("param AuditLog must be specified")
	case c.Authorizer == nil:
		return trace.BadParameter("param Authorizer must be specified")
	case c.Modules == nil:
		return trace.BadParameter("param Modules must be specified")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// Service implements resource usage gRPC server.
type Service struct {
	resourceusagepb.UnimplementedResourceUsageServiceServer

	auditLog   events.AuditLogger
	authorizer authz.Authorizer
	modules    modules.Modules
	clock      clockwork.Clock
}

// New creates a new Service according to the config.
func New(cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		auditLog:   cfg.AuditLog,
		authorizer: cfg.Authorizer,
		modules:    cfg.Modules,
		clock:      cfg.Clock,
	}, nil
}

// GetUsage implements resourceusagev1.ResourceUsageServiceServer.
func (s *Service) GetUsage(ctx context.Context, in *resourceusagepb.GetUsageRequest) (*resourceusagepb.GetUsageResponse, error) {
	if _, err := s.authorizer.Authorize(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	f := s.modules.Features()
	if !f.IsUsageBasedBilling {
		return resourceusagepb.GetUsageResponse_builder{
			AccountUsageType: resourceusagepb.AccountUsageType_ACCOUNT_USAGE_TYPE_UNLIMITED,
			AccessRequests:   &resourceusagepb.AccessRequestsUsage{},
		}.Build(), nil // unlimited
	}

	accessRequests, err := s.getAccessRequestsUsage(ctx, &f)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resourceusagepb.GetUsageResponse_builder{
		AccountUsageType: resourceusagepb.AccountUsageType_ACCOUNT_USAGE_TYPE_USAGE_BASED,
		AccessRequests:   accessRequests,
	}.Build(), nil
}

func (s *Service) getAccessRequestsUsage(ctx context.Context, f *modules.Features) (*resourceusagepb.AccessRequestsUsage, error) {
	monthlyUsed, err := resourceusage.GetAccessRequestMonthlyUsage(ctx, s.auditLog, s.clock.Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resourceusagepb.AccessRequestsUsage_builder{
		MonthlyLimit: f.GetEntitlement(entitlements.AccessRequests).Limit,
		MonthlyUsed:  int32(monthlyUsed),
	}.Build(), nil
}
