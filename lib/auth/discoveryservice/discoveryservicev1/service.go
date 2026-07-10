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

// Package discoveryservicev1 implements the discovery_service resource RPC
// service: Discovery Service configuration heartbeats. See RFD 253.
package discoveryservicev1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the DiscoveryServiceService gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger *slog.Logger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing discovery_service resources.
	Backend services.DiscoveryServices
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
	if s.Logger == nil {
		s.Logger = slog.With(teleport.ComponentKey, "discoveryservice_crud_service")
	}
	return nil
}

// Service implements the teleport.discoveryservice.v1.DiscoveryServiceService RPC service.
type Service struct {
	discoveryservicev1.UnimplementedDiscoveryServiceServiceServer

	log        *slog.Logger
	authorizer authz.Authorizer
	backend    services.DiscoveryServices
}

// NewService returns a new DiscoveryServiceService gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{
		log:        cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
	}, nil
}

// UpsertDiscoveryService announces a Discovery Service instance's presence and
// effective configuration. Only the Discovery builtin role may call it, and
// only for the resource named by the caller's own host ID; heartbeats are
// self-reports, never grants.
func (s *Service) UpsertDiscoveryService(ctx context.Context, req *discoveryservicev1.UpsertDiscoveryServiceRequest) (*discoveryservicev1.DiscoveryService, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleDiscovery)) {
		return nil, trace.AccessDenied("UpsertDiscoveryService request can be only executed by a Discovery Service")
	}
	role, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok {
		return nil, trace.AccessDenied("UpsertDiscoveryService request can be only executed by a Discovery Service")
	}

	svc := req.GetDiscoveryService()
	if name := svc.GetMetadata().GetName(); name != role.GetServerID() {
		return nil, trace.AccessDenied("a Discovery Service may only upsert its own discovery_service resource: got %q, expected %q", name, role.GetServerID())
	}

	// The heartbeat carries configuration only; kind/version are fixed and
	// status is reserved and never written through this RPC.
	svc.SetKind(types.KindDiscoveryService)
	svc.SetVersion(types.V1)
	svc.SetStatus(&discoveryservicev1.DiscoveryServiceStatus{})

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
