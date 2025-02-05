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
	"cmp"
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
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

	// Clock is used to control time - mainly used for testing.
	Clock clockwork.Clock

	// UsageReporter is the reporter for sending usage without it be related to an API call.
	UsageReporter func() usagereporter.UsageReporter

	// Emitter is the event emitter.
	Emitter apievents.Emitter
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
	if s.Emitter == nil {
		return trace.BadParameter("emitter is required")
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
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
	clock         clockwork.Clock
	usageReporter func() usagereporter.UsageReporter
	emitter       apievents.Emitter
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
		clock:         cfg.Clock,
		usageReporter: cfg.UsageReporter,
		emitter:       cfg.Emitter,
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

	s.updateStatus(req.UserTask, nil /* existing user task */)

	rsp, err := s.backend.CreateUserTask(ctx, req.UserTask)
	s.emitCreateAuditEvent(ctx, rsp, authCtx, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))

	return rsp, nil
}

func (s *Service) emitCreateAuditEvent(ctx context.Context, req *usertasksv1.UserTask, authCtx *authz.Context, createErr error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.UserTaskCreate{
		Metadata: apievents.Metadata{
			Type: libevents.UserTaskCreateEvent,
			Code: libevents.UserTaskCreateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(createErr),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      req.GetMetadata().GetName(),
			Expires:   getExpires(req.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		UserTaskMetadata: apievents.UserTaskMetadata{
			TaskType:    req.GetSpec().GetTaskType(),
			IssueType:   req.GetSpec().GetIssueType(),
			Integration: req.GetSpec().GetIntegration(),
		},
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit user task create event.", "error", auditErr)
	}
}

func userTaskToUserTaskStateEvent(ut *usertasksv1.UserTask) *usagereporter.UserTaskStateEvent {
	ret := &usagereporter.UserTaskStateEvent{
		TaskType:  ut.GetSpec().GetTaskType(),
		IssueType: ut.GetSpec().GetIssueType(),
		State:     ut.GetSpec().GetState(),
	}
	switch ut.GetSpec().GetTaskType() {
	case usertasks.TaskTypeDiscoverEC2:
		ret.InstancesCount = int32(len(ut.GetSpec().GetDiscoverEc2().GetInstances()))
	case usertasks.TaskTypeDiscoverEKS:
		ret.InstancesCount = int32(len(ut.GetSpec().GetDiscoverEks().GetClusters()))
	case usertasks.TaskTypeDiscoverRDS:
		ret.InstancesCount = int32(len(ut.GetSpec().GetDiscoverRds().GetDatabases()))
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

	stateChanged := existingUserTask.GetSpec().GetState() != req.GetUserTask().GetSpec().GetState()
	s.updateStatus(req.UserTask, existingUserTask)

	rsp, err := s.backend.UpdateUserTask(ctx, req.UserTask)
	s.emitUpdateAuditEvent(ctx, existingUserTask, req.GetUserTask(), authCtx, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if stateChanged {
		s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))
	}

	return rsp, nil
}

func (s *Service) emitUpdateAuditEvent(ctx context.Context, old, new *usertasksv1.UserTask, authCtx *authz.Context, updateErr error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.UserTaskUpdate{
		Metadata: apievents.Metadata{
			Type: libevents.UserTaskUpdateEvent,
			Code: libevents.UserTaskUpdateCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(updateErr),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      new.GetMetadata().GetName(),
			Expires:   getExpires(new.GetMetadata().GetExpires()),
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
		UserTaskMetadata: apievents.UserTaskMetadata{
			TaskType:    new.GetSpec().GetTaskType(),
			IssueType:   new.GetSpec().GetIssueType(),
			Integration: new.GetSpec().GetIntegration(),
		},
		CurrentUserTaskState: old.GetSpec().GetState(),
		UpdatedUserTaskState: new.GetSpec().GetState(),
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit user task update event.", "error", auditErr)
	}
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

	var stateChanged bool

	existingUserTask, err := s.backend.GetUserTask(ctx, req.GetUserTask().GetMetadata().GetName())
	switch {
	case trace.IsNotFound(err):
		stateChanged = true

	case err != nil:
		return nil, trace.Wrap(err)

	default:
		stateChanged = existingUserTask.GetSpec().GetState() != req.GetUserTask().GetSpec().GetState()
	}

	s.updateStatus(req.UserTask, existingUserTask)

	rsp, err := s.backend.UpsertUserTask(ctx, req.UserTask)
	s.emitUpsertAuditEvent(ctx, existingUserTask, req.GetUserTask(), authCtx, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if stateChanged {
		s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))
	}

	return rsp, nil
}

func (s *Service) updateStatus(ut *usertasksv1.UserTask, existing *usertasksv1.UserTask) {
	// Default status for UserTask.
	ut.Status = &usertasksv1.UserTaskStatus{
		LastStateChange: timestamppb.New(s.clock.Now()),
	}

	if existing != nil {
		// Inherit everything from existing UserTask.
		ut.Status.LastStateChange = cmp.Or(existing.GetStatus().GetLastStateChange(), ut.Status.LastStateChange)

		// Update specific values.
		if existing.GetSpec().GetState() != ut.GetSpec().GetState() {
			ut.Status.LastStateChange = timestamppb.New(s.clock.Now())
		}
	}
}

func (s *Service) emitUpsertAuditEvent(ctx context.Context, old, new *usertasksv1.UserTask, authCtx *authz.Context, err error) {
	if old == nil {
		s.emitCreateAuditEvent(ctx, new, authCtx, err)
		return
	}
	s.emitUpdateAuditEvent(ctx, old, new, authCtx, err)
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

	err = s.backend.DeleteUserTask(ctx, req.GetName())
	s.emitDeleteAuditEvent(ctx, req.GetName(), authCtx, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) emitDeleteAuditEvent(ctx context.Context, taskName string, authCtx *authz.Context, deleteErr error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.UserTaskDelete{
		Metadata: apievents.Metadata{
			Type: libevents.UserTaskDeleteEvent,
			Code: libevents.UserTaskDeleteCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(deleteErr),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      taskName,
			UpdatedBy: authCtx.Identity.GetIdentity().Username,
		},
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit user task delete event.", "error", auditErr)
	}
}

func eventStatus(err error) apievents.Status {
	var msg string
	if err != nil {
		msg = err.Error()
	}

	return apievents.Status{
		Success:     err == nil,
		Error:       msg,
		UserMessage: msg,
	}
}

func getExpires(cj *timestamppb.Timestamp) time.Time {
	if cj == nil {
		return time.Time{}
	}
	return cj.AsTime()
}
