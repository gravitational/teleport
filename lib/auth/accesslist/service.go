// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package accesslist

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// ServiceConfig is the service config for the Access Lists gRPC service.
type ServiceConfig struct {
	// Backend is the backend to use.
	Backend backend.Backend

	// Logger is the logger to use.
	Logger logrus.FieldLogger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// AccessLists is the access list service to use.
	AccessLists services.AccessLists
}

func (c *ServiceConfig) validateConfig() error {
	if c.Backend == nil {
		return trace.BadParameter("backend is missing")
	}

	if c.Logger == nil {
		c.Logger = logrus.New().WithField(trace.Component, "access_list_crud_service")
	}

	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}

	var err error
	if c.AccessLists == nil {
		c.AccessLists, err = local.NewAccessListService(c.Backend, c.Backend.Clock())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

type Service struct {
	accesslistv1.UnimplementedAccessListServiceServer

	log         logrus.FieldLogger
	authorizer  authz.Authorizer
	accessLists services.AccessLists
}

// NewService creates a new Access List gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.validateConfig(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:         cfg.Logger,
		authorizer:  cfg.Authorizer,
		accessLists: cfg.AccessLists,
	}, nil
}

// GetAccessLists returns a list of all access lists.
func (s *Service) GetAccessLists(ctx context.Context, _ *accesslistv1.GetAccessListsRequest) (*accesslistv1.GetAccessListsResponse, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, err := s.accessLists.GetAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessLists := make([]*accesslistv1.AccessList, len(results))
	for i, r := range results {
		accessLists[i] = conv.ToProto(r)
	}

	return &accesslistv1.GetAccessListsResponse{
		AccessLists: accessLists,
	}, nil
}

// GetAccessList returns the specified access list resource.
func (s *Service) GetAccessList(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := s.accessLists.GetAccessList(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(result), nil
}

// UpsertAccessList creates or updates an access list resource.
func (s *Service) UpsertAccessList(ctx context.Context, req *accesslistv1.UpsertAccessListRequest) (*accesslistv1.AccessList, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessList, err := conv.FromProto(req.GetAccessList())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	responseAccessList, err := s.accessLists.UpsertAccessList(ctx, accessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(responseAccessList), nil
}

// DeleteAccessList removes the specified access list resource.
func (s *Service) DeleteAccessList(ctx context.Context, req *accesslistv1.DeleteAccessListRequest) (*emptypb.Empty, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.accessLists.DeleteAccessList(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllAccessLists removes all access lists.
func (s *Service) DeleteAllAccessLists(ctx context.Context, _ *accesslistv1.DeleteAllAccessListsRequest) (*emptypb.Empty, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true, types.KindAccessList, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.accessLists.DeleteAllAccessLists(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
