package local

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/monitoring"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	MonitoringUserStatePrefix = "state/user"
	MonitoringRoleStatePrefix = "state/role"
)

// MonitoringService is the local implementation of the SecReports service.
type MonitoringService struct {
	log     logrus.FieldLogger
	clock   clockwork.Clock
	userSVC *generic.Service[*monitoring.User]
	roleSVC *generic.Service[*monitoring.Role]
}

// NewMonitoringService returns a new instance of the SecReports service.
func NewMonitoringService(backend backend.Backend, clock clockwork.Clock) (*MonitoringService, error) {
	userSVC, err := generic.NewService(&generic.ServiceConfig[*monitoring.User]{
		Backend:       backend,
		ResourceKind:  types.KindMonitoringUserState,
		BackendPrefix: MonitoringUserStatePrefix,
		MarshalFunc:   services.MarshalMonitoringUser,
		UnmarshalFunc: services.UnmarshalMonitoringUser,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleSVC, err := generic.NewService(&generic.ServiceConfig[*monitoring.Role]{
		Backend:       backend,
		ResourceKind:  types.KindMonitoringRoleState,
		BackendPrefix: MonitoringRoleStatePrefix,
		MarshalFunc:   services.MarshalMonitoringRole,
		UnmarshalFunc: services.UnmarshalMonitoringRole,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &MonitoringService{
		log:     logrus.WithFields(logrus.Fields{trace.Component: "monitoring:local-service"}),
		clock:   clock,
		userSVC: userSVC,
		roleSVC: roleSVC,
	}, nil
}

// UpsertMonitoringRole upserts audit query.
func (s *MonitoringService) UpsertMonitoringRole(ctx context.Context, in *monitoring.Role) error {
	if _, err := s.roleSVC.UpsertResource(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ListMonitoringRoles returns a list of audit queries.
func (s *MonitoringService) ListMonitoringRoles(ctx context.Context, pageSize int, nextToken string) ([]*monitoring.Role, string, error) {
	items, nextToken, err := s.roleSVC.ListResources(ctx, pageSize, nextToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextToken, nil
}

// GetMonitoringRole returns audit query by name.
func (s *MonitoringService) GetMonitoringRole(ctx context.Context, name string) (*monitoring.Role, error) {
	r, err := s.roleSVC.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}

// UpsertMonitoringUser upserts audit query.
func (s *MonitoringService) UpsertMonitoringUser(ctx context.Context, in *monitoring.User) error {
	if _, err := s.userSVC.UpsertResource(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ListMonitoringUsers returns a list of audit queries.
func (s *MonitoringService) ListMonitoringUsers(ctx context.Context, pageSize int, nextToken string) ([]*monitoring.User, string, error) {
	items, nextToken, err := s.userSVC.ListResources(ctx, pageSize, nextToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextToken, nil
}

// GetMonitoringUser returns audit query by name.
func (s *MonitoringService) GetMonitoringUser(ctx context.Context, name string) (*monitoring.User, error) {
	r, err := s.userSVC.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r, nil
}
