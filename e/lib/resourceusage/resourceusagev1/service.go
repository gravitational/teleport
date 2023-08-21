package resourceusagev1

import (
	context "context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/resourceusage"
)

// ServiceConfig contains parameters and dependencies for the resource usage Service.
type ServiceConfig struct {
	AuditLog   events.AuditLogger
	Authorizer authz.Authorizer
	Clock      clockwork.Clock
}

// checkAndSetDefaults checks and sets the defaults.
func (c *ServiceConfig) checkAndSetDefaults() error {
	if c.AuditLog == nil {
		return trace.BadParameter("AuditLog must be specified")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("Authorizer must be specified")
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
		clock:      cfg.Clock,
	}, nil
}

// GetUsage implements resourceusagev1.ResourceUsageServiceServer.
func (s *Service) GetUsage(ctx context.Context, in *resourceusagepb.GetUsageRequest) (*resourceusagepb.GetUsageResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if s.hasBuiltinRole(*authCtx, types.RoleNop) {
		return nil, trace.AccessDenied("resource usage information is not available to unauthenticated clients")
	}

	f := modules.GetModules().Features()
	if !f.IsUsageBasedBilling {
		return &resourceusagepb.GetUsageResponse{}, nil // unlimited
	}
	monthlyLimit := f.AccessRequests.MonthlyRequestLimit

	usage, err := resourceusage.GetAccessRequestMonthlyUsage(ctx, s.auditLog, s.clock.Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &resourceusagepb.GetUsageResponse{
		AccessRequests: &resourceusagepb.AccessRequestsUsage{
			MonthlyLimit: int32(monthlyLimit),
			MonthlyUsed:  int32(usage),
		},
	}, nil
}

// hasBuiltinRole checks that the attached identity is a builtin role and
// whether any of the given roles match the role set.
func (s *Service) hasBuiltinRole(ctx authz.Context, roles ...types.SystemRole) bool {
	for _, role := range roles {
		if authz.HasBuiltinRole(ctx, string(role)) {
			return true
		}
	}
	return false
}
