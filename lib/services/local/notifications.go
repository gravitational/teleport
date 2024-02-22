/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package local

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
)

// NotificationsService manages notification resources in the backend.
type NotificationsService struct {
	log                             logrus.FieldLogger
	userNotificationService         *generic.ServiceWrapper[*notificationsv1.Notification]
	globalNotificationService       *generic.ServiceWrapper[*notificationsv1.GlobalNotification]
	userNotificationStateService    *generic.ServiceWrapper[*notificationsv1.UserNotificationState]
	userLastSeenNotificationService *generic.ServiceWrapper[*notificationsv1.UserLastSeenNotification]
}

// NewNotificationsService returns a new isntance of the NotificationService.
func NewNotificationsService(backend backend.Backend, clock clockwork.Clock) (*NotificationsService, error) {
	userNotificationService, err := generic.NewServiceWrapper[*notificationsv1.Notification](backend, types.KindNotification, notificationsUserSpecificPrefix, services.MarshalNotification, services.UnmarshalNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	globalNotificationService, err := generic.NewServiceWrapper[*notificationsv1.GlobalNotification](backend, types.KindGlobalNotification, notificationsGlobalPrefix, services.MarshalGlobalNotification, services.UnmarshalGlobalNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userNotificationStateService, err := generic.NewServiceWrapper[*notificationsv1.UserNotificationState](backend, types.KindUserNotificationState, notificationsStatePrefix, services.MarshalUserNotificationState, services.UnmarshalUserNotificationState)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userLastSeenNotificationService, err := generic.NewServiceWrapper[*notificationsv1.UserLastSeenNotification](backend, types.KindUserLastSeenNotification, notificationsUserLastSeenPrefix, services.MarshalUserLastSeenNotification, services.UnmarshalUserLastSeenNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &NotificationsService{
		log:                             logrus.WithFields(logrus.Fields{trace.Component: "notifications:local-service"}),
		userNotificationService:         userNotificationService,
		globalNotificationService:       globalNotificationService,
		userNotificationStateService:    userNotificationStateService,
		userLastSeenNotificationService: userLastSeenNotificationService,
	}, nil
}

// ListDatabases returns a paginated list of notifications which match a user, including both user-specific and global ones.
func (s *NotificationsService) ListNotificationsForUser(ctx context.Context) ([]*notificationsv1.Notification, string, error) {
	// TODO: rudream - implement listing notifications for a user with filtering/matching
	return []*notificationsv1.Notification{}, "", nil
}

// CreateUserNotification creates a user-specific notification.
func (s *NotificationsService) CreateUserNotification(ctx context.Context, username string, notification *notificationsv1.Notification) (*notificationsv1.Notification, error) {
	if err := services.ValidateNotification(notification); err != nil {
		return nil, trace.Wrap(err)
	}

	notification.Kind = types.KindNotification
	notification.Version = types.V1

	// We set this to the UUID because the service adapter uses `getName()` to determine the backend key to use when storing the notification.
	notification.Metadata.Name = notification.Spec.Id

	if err := CheckAndSetExpiry(&notification.Metadata.Expires); err != nil {
		return nil, trace.Wrap(err)
	}

	// The `Created` field should always represent the time that the notification was created (ie. stored in the backend), it should thus only
	// ever be set here and should not be provided by the caller.
	// We do this check here instead of in `ValidateNotification` because `ValidateNotification` is also run by the marshaling function, and by that point
	// this field will have been populated.
	if notification.Spec.Created != nil {
		return nil, trace.Wrap(trace.BadParameter("notification created time should be empty"))
	}
	notification.Spec.Created = timestamppb.New(time.Now())

	// Append username prefix.
	serviceWithPrefix, err := s.userNotificationService.WithPrefix(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := serviceWithPrefix.CreateResource(ctx, notification)
	return created, trace.Wrap(err)
}

// DeleteUserNotification deletes a user-specific notification.
func (s *NotificationsService) DeleteUserNotification(ctx context.Context, username string, notificationId string) error {
	// Append username prefix.
	serviceWithPrefix, err := s.userNotificationService.WithPrefix(username)
	if err != nil {
		return trace.Wrap(err)
	}

	err = serviceWithPrefix.DeleteResource(ctx, notificationId)
	return trace.Wrap(err)
}

// DeleteAllUserNotificationsForUser deletes all of a user's user-specific notifications.
func (s *NotificationsService) DeleteAllUserNotificationsForUser(ctx context.Context, username string) error {
	// Append username prefix.
	serviceWithPrefix, err := s.userNotificationService.WithPrefix(username)
	if err != nil {
		return trace.Wrap(err)
	}

	err = serviceWithPrefix.DeleteAllResources(ctx)
	return trace.Wrap(err)
}

// CreateGlobalNotification creates a global notification.
func (s *NotificationsService) CreateGlobalNotification(ctx context.Context, globalNotification *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, error) {
	if err := services.ValidateGlobalNotification(globalNotification); err != nil {
		return nil, trace.Wrap(err)
	}

	globalNotification.Kind = types.KindGlobalNotification
	globalNotification.Version = types.V1

	if globalNotification.Metadata == nil {
		globalNotification.Metadata = &headerv1.Metadata{}
	}

	// We set this to the UUID because the service adapter uses `getName()` to determine the backend key to use when storing the notification.
	globalNotification.Metadata.Name = globalNotification.Spec.Notification.Spec.Id

	if err := CheckAndSetExpiry(&globalNotification.Spec.Notification.Metadata.Expires); err != nil {
		return nil, trace.Wrap(err)
	}

	if globalNotification.Spec.Notification.Spec.Created != nil {
		return nil, trace.Wrap(trace.BadParameter("notification created time should be empty"))
	}
	globalNotification.Spec.Notification.Spec.Created = timestamppb.New(time.Now())

	created, err := s.globalNotificationService.CreateResource(ctx, globalNotification)
	return created, trace.Wrap(err)
}

// DeleteGlobalNotification deletes a global notification.
func (s *NotificationsService) DeleteGlobalNotification(ctx context.Context, notificationId string) error {
	err := s.globalNotificationService.DeleteResource(ctx, notificationId)
	return trace.Wrap(err)
}

// UpsertUserNotificationState creates or updates a user notification state which records whether the user has clicked on or dismissed a notification.
func (s *NotificationsService) UpsertUserNotificationState(ctx context.Context, username string, state *notificationsv1.UserNotificationState) (*notificationsv1.UserNotificationState, error) {
	if err := services.ValidateUserNotificationState(state); err != nil {
		return nil, trace.Wrap(err)
	}

	state.Kind = types.KindUserNotificationState
	state.Version = types.V1

	if state.Metadata == nil {
		state.Metadata = &headerv1.Metadata{}
	}

	// We set this to the notification UUID because the service adapter uses `getName()` to determine the backend key to use when storing the object.
	state.Metadata.Name = state.Spec.NotificationId

	// Append username prefix.
	serviceWithPrefix, err := s.userNotificationStateService.WithPrefix(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := serviceWithPrefix.UpsertResource(ctx, state)
	return upserted, trace.Wrap(err)
}

// DeleteUserNotificationState deletes a user notification state object.
func (s *NotificationsService) DeleteUserNotificationState(ctx context.Context, username string, notificationId string) error {
	// Append username prefix.
	serviceWithPrefix, err := s.userNotificationStateService.WithPrefix(username)
	if err != nil {
		return trace.Wrap(err)
	}

	err = serviceWithPrefix.DeleteResource(ctx, notificationId)
	return trace.Wrap(err)
}

// DeleteAllUserNotificationStatesForUser deletes all of a user's notification states.
func (s *NotificationsService) DeleteAllUserNotificationStatesForUser(ctx context.Context, username string) error {
	// Append username prefix.
	serviceWithPrefix, err := s.userNotificationStateService.WithPrefix(username)
	if err != nil {
		return trace.Wrap(err)
	}

	err = serviceWithPrefix.DeleteAllResources(ctx)
	return trace.Wrap(err)
}

// ListUserNotificationStates returns a page of a user's notification states.
func (s *NotificationsService) ListUserNotificationStates(ctx context.Context, username string, pageSize int, nextToken string) ([]*notificationsv1.UserNotificationState, string, error) {
	// Append username prefix.
	serviceWithPrefix, err := s.userNotificationStateService.WithPrefix(username)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	states, nextToken, err := serviceWithPrefix.ListResources(ctx, pageSize, nextToken)
	return states, nextToken, trace.Wrap(err)
}

// UpsertUserLastSeenNotification creates or updates a user's last seen notification item.
func (s *NotificationsService) UpsertUserLastSeenNotification(ctx context.Context, username string, ulsn *notificationsv1.UserLastSeenNotification) (*notificationsv1.UserLastSeenNotification, error) {
	if err := services.ValidateUserLastSeenNotification(ulsn); err != nil {
		return nil, trace.Wrap(err)
	}

	ulsn.Kind = types.KindUserLastSeenNotification
	ulsn.Version = types.V1

	if ulsn.Metadata == nil {
		ulsn.Metadata = &headerv1.Metadata{}
	}
	// We set this to the username because the service adapter uses `getName()` to determine the backend key to use when storing the object.
	ulsn.Metadata.Name = username

	upserted, err := s.userLastSeenNotificationService.UpsertResource(ctx, ulsn)
	return upserted, trace.Wrap(err)
}

// GetUserLastSeenNotification returns a user's last seen notification item.
func (s *NotificationsService) GetUserLastSeenNotification(ctx context.Context, username string) (*notificationsv1.UserLastSeenNotification, error) {

	ulsn, err := s.userLastSeenNotificationService.GetResource(ctx, username)
	return ulsn, trace.Wrap(err)
}

// DeleteUserLastSeenNotification deletes a user's last seen notification item.
func (s *NotificationsService) DeleteUserLastSeenNotification(ctx context.Context, username string) error {
	err := s.userLastSeenNotificationService.DeleteResource(ctx, username)
	return trace.Wrap(err)
}

// CheckAndSetExpiry checks and sets the default expiry for a notification.
func CheckAndSetExpiry(expires **timestamppb.Timestamp) error {
	// If the expiry hasn't been provided, set the default to 30 days from now.
	if *expires == nil {
		now := time.Now()
		futureTime := now.Add(30 * 24 * time.Hour)
		*expires = timestamppb.New(futureTime)
	}

	// If the expiry has already been provided, ensure that it is not more than 90 days from now.
	// This is to prevent misuse as we don't want notifications existing for too long and accumulating in the backend.
	now := time.Now()
	maxExpiry := now.Add(90 * 24 * time.Hour)

	if (*expires).AsTime().After(maxExpiry) {
		return trace.BadParameter("notification expiry cannot be more than 90 days from its creation")
	}

	return nil
}

const (
	notificationsGlobalPrefix       = "notifications/global"    // notifications/global/<notification id>
	notificationsUserSpecificPrefix = "notifications/user"      // notifications/user/<username>/<notification id>
	notificationsStatePrefix        = "notifications/states"    // notifications/states/<username>/<notification id>
	notificationsUserLastSeenPrefix = "notifications/last_seen" // notifications/last_seen/<username>
)
