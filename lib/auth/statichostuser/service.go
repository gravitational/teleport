// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package statichostuser

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	convertv1 "github.com/gravitational/teleport/api/types/userprovisioning/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the static host user gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// Backend is the backend used to store static host users.
	Backend services.StaticHostUser
	// Cache is the cache used to store static host users.
	Cache Cache
}

// Cache is a subset of the service interface for reading items from the cache.
type Cache interface {
	// ListStaticHostUsers lists static host users.
	ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioning.StaticHostUser, string, error)
	// GetStaticHostUser returns a static host user by name.
	GetStaticHostUser(ctx context.Context, name string) (*userprovisioning.StaticHostUser, error)
}

// Service implements the static host user RPC service.
type Service struct {
	userprovisioningpb.UnimplementedStaticHostUsersServiceServer

	authorizer authz.Authorizer
	backend    services.StaticHostUser
	cache      Cache
}

// NewService creates a new static host user gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
	}, nil
}

// ListStaticHostUsers lists static host users.
func (s *Service) ListStaticHostUsers(ctx context.Context, req *userprovisioningpb.ListStaticHostUsersRequest) (*userprovisioningpb.ListStaticHostUsersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	users, nextToken, err := s.cache.ListStaticHostUsers(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userProtos := make([]*userprovisioningpb.StaticHostUser, 0, len(users))
	for _, u := range users {
		userProtos = append(userProtos, convertv1.ToProto(u))
	}
	return &userprovisioningpb.ListStaticHostUsersResponse{
		Users:         userProtos,
		NextPageToken: nextToken,
	}, nil
}

// GetStaticHostUser returns a static host user by name.
func (s *Service) GetStaticHostUser(ctx context.Context, req *userprovisioningpb.GetStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("missing name")
	}
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.cache.GetStaticHostUser(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convertv1.ToProto(out), nil
}

// CreateStaticHostUser creates a static host user.
func (s *Service) CreateStaticHostUser(ctx context.Context, req *userprovisioningpb.CreateStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	hostUser, err := convertv1.FromProto(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := s.backend.CreateStaticHostUser(ctx, hostUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convertv1.ToProto(out), trace.Wrap(err)
}

// UpdateStaticHostUser updates a static host user.
func (s *Service) UpdateStaticHostUser(ctx context.Context, req *userprovisioningpb.UpdateStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	hostUser, err := convertv1.FromProto(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := s.backend.UpdateStaticHostUser(ctx, hostUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convertv1.ToProto(out), trace.Wrap(err)
}

// UpsertStaticHostUser upserts a static host user.
func (s *Service) UpsertStaticHostUser(ctx context.Context, req *userprovisioningpb.UpsertStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	hostUser, err := convertv1.FromProto(req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := s.backend.UpsertStaticHostUser(ctx, hostUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convertv1.ToProto(out), trace.Wrap(err)
}

// DeleteStaticHostUser deletes a static host user. Note that this does not
// remove any host users created on nodes from the resource.
func (s *Service) DeleteStaticHostUser(ctx context.Context, req *userprovisioningpb.DeleteStaticHostUserRequest) (*emptypb.Empty, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("missing name")
	}
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(s.backend.DeleteStaticHostUser(ctx, req.Name))
}
