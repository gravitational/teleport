// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statichostuser

import (
	"context"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.StaticHostUser
}

type Service struct {
	userprovisioningpb.UnimplementedStaticHostUsersServiceServer

	authorizer authz.Authorizer
	backend    services.StaticHostUser
}

func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
	}, nil
}

func (s *Service) ListStaticHostUsers(ctx context.Context, req *userprovisioningpb.ListStaticHostUsersRequest) (*userprovisioningpb.ListStaticHostUsersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	users, nextToken, err := s.backend.ListStaticHostUsers(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &userprovisioningpb.ListStaticHostUsersResponse{
		Users:         users,
		NextPageToken: nextToken,
	}, nil
}

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
	out, err := s.backend.GetStaticHostUser(ctx, req.Name)
	return out, trace.Wrap(err)
}

func (s *Service) CreateStaticHostUser(ctx context.Context, req *userprovisioningpb.CreateStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := s.backend.CreateStaticHostUser(ctx, req.User)
	return out, trace.Wrap(err)
}

func (s *Service) UpdateStaticHostUser(ctx context.Context, req *userprovisioningpb.UpdateStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := s.backend.UpdateStaticHostUser(ctx, req.User)
	return out, trace.Wrap(err)
}

func (s *Service) UpsertStaticHostUser(ctx context.Context, req *userprovisioningpb.UpsertStaticHostUserRequest) (*userprovisioningpb.StaticHostUser, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindStaticHostUser, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := s.backend.UpsertStaticHostUser(ctx, req.User)
	return out, trace.Wrap(err)
}

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
	return &emptypb.Empty{}, trace.Wrap(s.backend.DeleteStaticHostUser(ctx, req.Name))
}
