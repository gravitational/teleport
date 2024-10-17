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
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// ServiceConfig holds configuration options for the UserTask gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing UserTask.
	Backend services.UserTasks

	// Cache is the cache for storing UserTask.
	Cache Reader

	// UsageReporter is the reporter for sending usage without it be related to an API call.
	UsageReporter func() usagereporter.UsageReporter
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
	if s.UsageReporter == nil {
		return trace.BadParameter("usage reporter is required")
	}

	return nil
}

// Reader contains the methods defined for cache access.
type Reader interface {
	ListUserTasks(ctx context.Context, pageSize int64, nextToken string) ([]*usertasksv1.UserTask, string, error)
	ListUserTasksByIntegration(ctx context.Context, pageSize int64, nextToken string, integration string) ([]*usertasksv1.UserTask, string, error)
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
}

// Service implements the teleport.UserTask.v1.UserTaskService RPC service.
type Service struct {
	usertasksv1.UnimplementedUserTaskServiceServer

	authorizer    authz.Authorizer
	backend       services.UserTasks
	cache         Reader
	usageReporter func() usagereporter.UsageReporter
}

// NewService returns a new UserTask gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		authorizer:    cfg.Authorizer,
		backend:       cfg.Backend,
		cache:         cfg.Cache,
		usageReporter: cfg.UsageReporter,
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

	s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))

	return rsp, nil
}

func userTaskToUserTaskStateEvent(ut *usertasksv1.UserTask) *usagereporter.UserTaskStateEvent {
	ret := &usagereporter.UserTaskStateEvent{
		TaskType:  ut.GetSpec().GetTaskType(),
		IssueType: ut.GetSpec().GetTaskType(),
		State:     ut.GetSpec().GetState(),
	}
	if ut.GetSpec().GetTaskType() == usertasks.TaskTypeDiscoverEC2 {
		ret.InstancesCount = int32(len(ut.GetSpec().GetDiscoverEc2().GetInstances()))
	}
	return ret
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

// ListUserTasksByIntegration returns a list of user tasks filtered by an integration.
func (s *Service) ListUserTasksByIntegration(ctx context.Context, req *usertasksv1.ListUserTasksByIntegrationRequest) (*usertasksv1.ListUserTasksResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, nextToken, err := s.cache.ListUserTasksByIntegration(ctx, req.PageSize, req.PageToken, req.Integration)
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

	existingUserTask, err := s.backend.GetUserTask(ctx, req.GetUserTask().GetMetadata().GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateUserTask(ctx, req.UserTask)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if existingUserTask.GetSpec().GetState() != req.GetUserTask().GetSpec().GetState() {
		s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))
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

	var emitStateChangeEvent bool

	existingUserTask, err := s.backend.GetUserTask(ctx, req.GetUserTask().GetMetadata().GetName())
	switch {
	case trace.IsNotFound(err):
		emitStateChangeEvent = true

	case err != nil:
		return nil, trace.Wrap(err)

	default:
		emitStateChangeEvent = existingUserTask.GetSpec().GetState() != req.GetUserTask().GetSpec().GetState()
	}

	rsp, err := s.backend.UpsertUserTask(ctx, req.UserTask)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if emitStateChangeEvent {
		s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))
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
