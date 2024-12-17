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
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/notifications"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// BackendService contains the methods used to manage Resources.
type BackendService interface {
	// CRUD methods for the UserTasks resources.
	services.UserTasks

	// Methods required to notify the users of pending User Tasks.
	CreateGlobalNotification(ctx context.Context, globalNotification *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, error)
	DeleteGlobalNotification(ctx context.Context, notificationId string) error
	UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error)
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
}

// ServiceConfig holds configuration options for the UserTask gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing resources.
	Backend BackendService

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
	backend       BackendService
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
// A global notification is created to ensure the user is aware of the pending user task.
func (s *Service) CreateUserTask(ctx context.Context, req *usertasksv1.CreateUserTaskRequest) (*usertasksv1.UserTask, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserTask, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	s.updateStatus(req.UserTask)

	rsp, err := s.backend.CreateUserTask(ctx, req.UserTask)
	s.emitCreateAuditEvent(ctx, rsp, authCtx, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))

	if err := s.notifyUserAboutPendingTask(ctx, rsp); err != nil {
		return nil, trace.Wrap(err)
	}

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
// If the UserTask state goes from Resolved to Open, a global notification is created to ensure the user is aware of the pending user task.
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

	if stateChanged {
		s.updateStatus(req.UserTask)
	}

	rsp, err := s.backend.UpdateUserTask(ctx, req.UserTask)
	s.emitUpdateAuditEvent(ctx, existingUserTask, req.GetUserTask(), authCtx, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if stateChanged {
		s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))
	}

	if err := s.notifyUserAboutPendingTask(ctx, rsp); err != nil {
		return nil, trace.Wrap(err)
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
// If the UserTask state goes from Resolved to Open, a global notification is created to ensure the user is aware of the pending user task.
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

	if stateChanged {
		s.updateStatus(req.UserTask)
	}

	rsp, err := s.backend.UpsertUserTask(ctx, req.UserTask)
	s.emitUpsertAuditEvent(ctx, existingUserTask, req.GetUserTask(), authCtx, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if stateChanged {
		s.usageReporter().AnonymizeAndSubmit(userTaskToUserTaskStateEvent(req.GetUserTask()))
	}

	if err := s.notifyUserAboutPendingTask(ctx, rsp); err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

func (s *Service) updateStatus(ut *usertasksv1.UserTask) {
	ut.Status = &usertasksv1.UserTaskStatus{
		LastStateChange: timestamppb.New(s.clock.Now()),
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

// notifyUserAboutPendingTask creates a global notification that notifies the user about pending tasks.
// Only one notification per Integration is created.
// If a notification already exists, it will be deleted and re-created.
// When creating the notification, the longest lifespan of the existing and the new notification will be used.
func (s *Service) notifyUserAboutPendingTask(ctx context.Context, ut *usertasksv1.UserTask) error {
	if ut.GetSpec().GetState() != usertasks.TaskStateOpen {
		return nil
	}

	integrationName := ut.GetSpec().GetIntegration()
	expires := ut.GetMetadata().GetExpires().AsTime()

	integration, err := s.backend.GetIntegration(ctx, integrationName)
	if err != nil {
		return trace.Wrap(err)
	}
	integrationStatus := integration.GetStatus()
	if integrationStatus.PendingUserTasksNotificationID != "" {
		if err := s.backend.DeleteGlobalNotification(ctx, integrationStatus.PendingUserTasksNotificationID); err != nil {
			// NotFound might be returned when the GlobalNotification already expired or was deleted.
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}

		if integrationStatus.PendingUserTasksNotificationExpires != nil {
			// Ensure we keep the longest lived notification.
			if expires.Before(*integrationStatus.PendingUserTasksNotificationExpires) {
				expires = *integrationStatus.PendingUserTasksNotificationExpires
			}
		}
	}

	pendingUserTasksNotification := notifications.NewPendingUserTasksIntegrationNotification(integrationName, expires)
	newNotification, err := s.backend.CreateGlobalNotification(ctx, pendingUserTasksNotification)
	if err != nil {
		return trace.Wrap(err)
	}

	integration.SetStatus(types.IntegrationStatusV1{
		PendingUserTasksNotificationID:      newNotification.GetMetadata().GetName(),
		PendingUserTasksNotificationExpires: &expires,
	})

	if _, err := s.backend.UpdateIntegration(ctx, integration); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
