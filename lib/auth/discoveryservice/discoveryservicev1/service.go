// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package discoveryservicev1 implements the Auth RPC service for the discovery_service
// resource: TTL-backed heartbeats in which each Discovery Service instance self-reports
// its identity and the static configuration loaded from its own teleport.yaml
// (discovery group, poll interval, static matchers). Dynamic DiscoveryConfig contents are never
// part of a heartbeat; readers correlate the two resources by exact discovery_group equality.
package discoveryservicev1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the DiscoveryHeartbeatService gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing discovery_service resources.
	Backend services.DiscoveryServices

	// Clock is used to assign record expiry; defaults to the real clock.
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	return nil
}

// Service implements the teleport.discoveryservice.v1.DiscoveryHeartbeatService RPC service.
type Service struct {
	discoveryservicev1.UnimplementedDiscoveryHeartbeatServiceServer

	authorizer authz.Authorizer
	backend    services.DiscoveryServices
	clock      clockwork.Clock
}

// NewService returns a new DiscoveryHeartbeatService gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		clock:      cfg.Clock,
	}, nil
}

// UpsertDiscoveryService announces a Discovery Service instance's presence and effective
// static configuration. Only the Discovery builtin role may call it, and only for the
// resource named by the caller's own host ID; heartbeats are self-reports, never grants.
func (s *Service) UpsertDiscoveryService(ctx context.Context, req *discoveryservicev1.UpsertDiscoveryServiceRequest) (*discoveryservicev1.DiscoveryService, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	role, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok || !authz.HasBuiltinRole(*authCtx, string(types.RoleDiscovery)) {
		return nil, trace.AccessDenied("UpsertDiscoveryService request can be only executed by a Discovery Service")
	}

	if req == nil || req.GetDiscoveryService() == nil {
		return nil, trace.BadParameter("missing discovery service")
	}
	submitted := req.GetDiscoveryService()
	if submitted.GetMetadata() == nil {
		return nil, trace.BadParameter("missing metadata")
	}
	if submitted.GetSpec() == nil {
		return nil, trace.BadParameter("missing spec")
	}
	svcName := submitted.GetMetadata().GetName()
	if svcName == "" {
		return nil, trace.BadParameter("missing name")
	}
	if svcName != role.GetServerID() {
		return nil, trace.AccessDenied("a Discovery Service may only upsert its own discovery_service resource: got %q, expected %q", svcName, role.GetServerID())
	}

	// Reconstruct the complete envelope so no caller-controlled metadata or
	// status reaches storage. The spec is the only agent-owned payload.
	svc := discoveryservicev1.DiscoveryService_builder{
		Kind:    types.KindDiscoveryService,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:    role.GetServerID(),
			Expires: timestamppb.New(s.clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL)),
		}.Build(),
		Spec:   proto.CloneOf(submitted.GetSpec()),
		Status: &discoveryservicev1.DiscoveryServiceStatus{},
	}.Build()

	upserted, err := s.backend.UpsertDiscoveryService(ctx, svc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return upserted, nil
}

// GetDiscoveryService returns the specified discovery_service resource.
func (s *Service) GetDiscoveryService(ctx context.Context, req *discoveryservicev1.GetDiscoveryServiceRequest) (*discoveryservicev1.DiscoveryService, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDiscoveryService, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	svc, err := s.backend.GetDiscoveryService(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return svc, nil
}

// ListDiscoveryServices returns a paginated list of discovery_service resources.
func (s *Service) ListDiscoveryServices(ctx context.Context, req *discoveryservicev1.ListDiscoveryServicesRequest) (*discoveryservicev1.ListDiscoveryServicesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDiscoveryService, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextPageToken, err := s.backend.ListDiscoveryServices(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return discoveryservicev1.ListDiscoveryServicesResponse_builder{
		DiscoveryServices: results,
		NextPageToken:     nextPageToken,
	}.Build(), nil
}

// DeleteDiscoveryService removes the specified discovery_service resource.
func (s *Service) DeleteDiscoveryService(ctx context.Context, req *discoveryservicev1.DeleteDiscoveryServiceRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDiscoveryService, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteDiscoveryService(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
