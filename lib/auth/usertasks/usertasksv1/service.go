/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package usertasksv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the UserTask gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing UserTask.
	Backend services.UserTasks

	// Cache is the cache for storing UserTask.
	Cache Reader
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if s.Cache == nil {
		return trace.BadParameter("cache is required")
	}

	return nil
}

// Reader contains the methods defined for cache access.
type Reader interface {
	ListUserTasks(ctx context.Context, pageSize int64, nextToken string) ([]*usertasksv1.UserTask, string, error)
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
}

// Service implements the teleport.UserTask.v1.UserTaskService RPC service.
type Service struct {
	usertasksv1.UnimplementedUserTaskServiceServer

	authorizer authz.Authorizer
	backend    services.UserTasks
	cache      Reader
}

// NewService returns a new UserTask gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
	}, nil
}

// CreateUserTask creates user task resource.
func (s *Service) CreateUserTask(ctx context.Context, req *usertasksv1.CreateUserTaskRequest) (*usertasksv1.UserTask, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.CreateUserTask(ctx, req.UserTask)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// ListUserTasks returns a list of user tasks.
func (s *Service) ListUserTasks(ctx context.Context, req *usertasksv1.ListUserTasksRequest) (*usertasksv1.ListUserTasksResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, nextToken, err := s.cache.ListUserTasks(ctx, req.PageSize, req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &usertasksv1.ListUserTasksResponse{
		UserTasks:     rsp,
		NextPageToken: nextToken,
	}, nil
}

// GetUserTask returns user task resource.
func (s *Service) GetUserTask(ctx context.Context, req *usertasksv1.GetUserTaskRequest) (*usertasksv1.UserTask, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.cache.GetUserTask(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil

}

// UpdateUserTask updates user task resource.
func (s *Service) UpdateUserTask(ctx context.Context, req *usertasksv1.UpdateUserTaskRequest) (*usertasksv1.UserTask, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateUserTask(ctx, req.UserTask)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// UpsertUserTask upserts user task resource.
func (s *Service) UpsertUserTask(ctx context.Context, req *usertasksv1.UpsertUserTaskRequest) (*usertasksv1.UserTask, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbUpdate, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpsertUserTask(ctx, req.UserTask)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil

}

// DeleteUserTask deletes user task resource.
func (s *Service) DeleteUserTask(ctx context.Context, req *usertasksv1.DeleteUserTaskRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteUserTask(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
