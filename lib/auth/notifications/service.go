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

package notifications

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the notifications gRPC service.
type ServiceConfig struct {
	// Backend is the backend used to store Kubernetes waiting containers.
	Backend services.Notifications

	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
}

// Service implements the teleport.notications.v1.NotificationsService RPC Service.
type Service struct {
	notificationsv1.UnimplementedNotificationServiceServer

	authorizer authz.Authorizer
	backend    services.Notifications
}

// NewService returns a new notificationns gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("server with roles is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
	}, nil
}

func (s *Service) ListNotifications(context.Context, *notificationsv1.ListNotificationsRequest) (*notificationsv1.ListNotificationsResponse, error) {
	fmt.Printf("\n\ni want to call authwithroles here not the backend\n\n")
	return nil, nil
}

// CreateGlobalNotification creates a global notification.
func (s *Service) CreateGlobalNotification(ctx context.Context, req *notificationsv1.CreateGlobalNotificationRequest) (*notificationsv1.GlobalNotification, error) {
	if req.GlobalNotification == nil {
		return nil, trace.BadParameter("missing notification")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindGlobalNotification, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.CreateGlobalNotification(ctx, req.GlobalNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// CreateUserNotification creates a user-specific notification.
func (s *Service) CreateUserNotification(ctx context.Context, req *notificationsv1.CreateUserNotificationRequest) (*notificationsv1.Notification, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.Notification == nil {
		return nil, trace.BadParameter("missing notification")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindNotification, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.CreateUserNotification(ctx, req.Notification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// DeleteGlobalNotification deletes a global notification.
func (s *Service) DeleteGlobalNotification(ctx context.Context, req *notificationsv1.DeleteGlobalNotificationRequest) (*emptypb.Empty, error) {
	if req.NotificationId == "" {
		return nil, trace.BadParameter("missing notification id")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindGlobalNotification, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteGlobalNotification(ctx, req.NotificationId); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteUserNotification deletes a user-specific notification.
func (s *Service) DeleteUserNotification(ctx context.Context, req *notificationsv1.DeleteUserNotificationRequest) (*emptypb.Empty, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.NotificationId == "" {
		return nil, trace.BadParameter("missing notification id")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()
	if username != req.Username {
		return nil, trace.AccessDenied("a user may only delete their own user-specific notifications")
	}

	if err := authCtx.CheckAccessToKind(types.KindNotification, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteUserNotification(ctx, req.Username, req.NotificationId); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// UpsertUserNotificationState creates or updates a user notification state which records whether the user has clicked on or dismissed a notification.
func (s *Service) UpsertUserNotificationState(ctx context.Context, req *notificationsv1.UpsertUserNotificationStateRequest) (*notificationsv1.UserNotificationState, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.UserNotificationState == nil {
		return nil, trace.BadParameter("missing notification state")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := authCtx.User.GetName()
	if username != req.Username {
		return nil, trace.AccessDenied("a user may only update their own notification state")
	}

	if err := authCtx.CheckAccessToKind(types.KindUserNotificationState, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.UpsertUserNotificationState(ctx, req.Username, req.UserNotificationState)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// UpsertUserLastSeenNotification creates or updates a user's last seen notification item.
func (s *Service) UpsertUserLastSeenNotification(ctx context.Context, req *notificationsv1.UpsertUserLastSeenNotificationRequest) (*notificationsv1.UserLastSeenNotification, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.UserLastSeenNotification == nil {
		return nil, trace.BadParameter("missing user last seen notification")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := authCtx.User.GetName()
	if username != req.Username {
		return nil, trace.AccessDenied("a user may only update their own last seen notification timestamp")
	}

	if err := authCtx.CheckAccessToKind(types.KindUserLastSeenNotification, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.UpsertUserLastSeenNotification(ctx, req.Username, req.UserLastSeenNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}
