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

package local

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// NotificationsService manages notification resources in the backend.
type NotificationsService struct {
	clock                           clockwork.Clock
	userNotificationService         *generic.ServiceWrapper[*notificationsv1.Notification]
	globalNotificationService       *generic.ServiceWrapper[*notificationsv1.GlobalNotification]
	userNotificationStateService    *generic.ServiceWrapper[*notificationsv1.UserNotificationState]
	userLastSeenNotificationService *generic.ServiceWrapper[*notificationsv1.UserLastSeenNotification]
}

// NewNotificationsService returns a new instance of the NotificationService.
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
		clock:                           clock,
		userNotificationService:         userNotificationService,
		globalNotificationService:       globalNotificationService,
		userNotificationStateService:    userNotificationStateService,
		userLastSeenNotificationService: userLastSeenNotificationService,
	}, nil
}

// ListUserNotifications returns a paginated list of user-specific notifications for all users.
func (s *NotificationsService) ListUserNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.Notification, string, error) {
	if pageSize < 1 {
		pageSize = apidefaults.DefaultChunkSize
	}

	if pageSize > apidefaults.DefaultChunkSize {
		return nil, "", trace.BadParameter("pageSize of %d is too large", pageSize)
	}

	resp, nextKey, err := s.userNotificationService.ListResources(ctx, pageSize, startKey)
	return resp, nextKey, trace.Wrap(err)
}

// DeleteAllUserNotifications deletes all user-specific notifications for all users. This should only be used by the cache.
func (s *NotificationsService) DeleteAllUserNotifications(ctx context.Context) error {
	if err := s.userNotificationService.DeleteAllResources(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ListGlobalNotifications returns a paginated list of global notifications.
func (s *NotificationsService) ListGlobalNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.GlobalNotification, string, error) {
	if pageSize < 1 {
		pageSize = apidefaults.DefaultChunkSize
	}

	if pageSize > apidefaults.DefaultChunkSize {
		return nil, "", trace.BadParameter("pageSize of %d is too large", pageSize)
	}

	resp, nextKey, err := s.globalNotificationService.ListResources(ctx, pageSize, startKey)
	return resp, nextKey, trace.Wrap(err)
}

// DeleteAllGlobalNotifications deletes all global notifications. This should only be used by the cache.
func (s *NotificationsService) DeleteAllGlobalNotifications(ctx context.Context) error {
	if err := s.globalNotificationService.DeleteAllResources(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CreateUserNotification creates a user-specific notification.
func (s *NotificationsService) CreateUserNotification(ctx context.Context, notification *notificationsv1.Notification) (*notificationsv1.Notification, error) {
	if err := services.ValidateNotification(notification); err != nil {
		return nil, trace.Wrap(err)
	}

	if notification.Spec.Username == "" {
		return nil, trace.BadParameter("a username must be specified")
	}

	notification.Kind = types.KindNotification
	notification.Version = types.V1

	// Generate uuidv7 ID.
	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	notification.Spec.Id = uuid.String()

	// We set this to the UUID because the service adapter uses `getName()` to determine the backend key to use when storing the notification.
	notification.Metadata.Name = notification.Spec.Id

	if err := CheckAndSetExpiry(notification, s.clock); err != nil {
		return nil, trace.Wrap(err)
	}

	notification.Spec.Created = timestamppb.New(s.clock.Now())

	// Append username prefix.
	serviceWithPrefix := s.userNotificationService.WithPrefix(notification.Spec.Username)

	created, err := serviceWithPrefix.CreateResource(ctx, notification)
	return created, trace.Wrap(err)
}

// UpsertUserNotification upserts a user notification resource that has already had its contents validated and its defaults such as the generated UUID, created date, and expiry date set.
func (s *NotificationsService) UpsertUserNotification(ctx context.Context, notification *notificationsv1.Notification) (*notificationsv1.Notification, error) {
	if err := services.ValidateNotification(notification); err != nil {
		return nil, trace.Wrap(err)
	}

	// Precautionary check in case of accidental misuse.
	if notification.Spec.Id == "" {
		return nil, trace.BadParameter("notification id is missing. Did you mean to use CreateUserNotification?")
	}

	// Append username prefix.
	serviceWithPrefix := s.userNotificationService.WithPrefix(notification.Spec.Username)

	created, err := serviceWithPrefix.UpsertResource(ctx, notification)
	return created, trace.Wrap(err)
}

// DeleteUserNotification deletes a user-specific notification.
func (s *NotificationsService) DeleteUserNotification(ctx context.Context, username string, notificationId string) error {
	// Append username prefix.
	serviceWithPrefix := s.userNotificationService.WithPrefix(username)

	// Delete the notification
	if err := serviceWithPrefix.DeleteResource(ctx, notificationId); err != nil {
		return trace.Wrap(err)
	}

	// Also delete the user notification state for this notification.
	notificationStateServiceWithPrefix := s.userNotificationStateService.WithPrefix(username)

	if err := notificationStateServiceWithPrefix.DeleteResource(ctx, notificationId); err != nil {
		// If the error is due to the user notification state not being found, then ignore it because
		// it is possible that it doesn't exist (if the user never clicked on or dismissed the notification).
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllUserNotificationsForUser deletes all of a user's user-specific notifications.
func (s *NotificationsService) DeleteAllUserNotificationsForUser(ctx context.Context, username string) error {
	// Append username prefix.
	serviceWithPrefix := s.userNotificationService.WithPrefix(username)

	err := serviceWithPrefix.DeleteAllResources(ctx)
	return trace.Wrap(err)
}

// CreateGlobalNotification creates a global notification.
func (s *NotificationsService) CreateGlobalNotification(ctx context.Context, globalNotification *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, error) {
	if err := services.ValidateGlobalNotification(globalNotification); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check to ensure that the metadata for the globalNotification isn't configured, this shouldn't be used and if it is configured, the caller likely meant to
	// configure the notification's metadata, which is in spec.notification.metadata.
	// We do this check here instead of in `ValidateGlobalNotification` because we only want to do this check on creation.
	if globalNotification.Metadata != nil {
		return nil, trace.BadParameter("metadata should be nil, metadata for a notification should be in spec.notification.metadata")
	}

	globalNotification.Kind = types.KindGlobalNotification
	globalNotification.Version = types.V1

	// Generate uuidv7 ID.
	uuid, err := uuid.NewV7()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	globalNotification.Spec.Notification.Spec.Id = uuid.String()

	// We set this to the UUID because the service adapter uses `getName()` to determine the backend key to use when storing the notification.
	globalNotification.Metadata = &headerv1.Metadata{Name: globalNotification.Spec.Notification.Spec.Id}

	if err := CheckAndSetExpiry(globalNotification.Spec.Notification, s.clock); err != nil {
		return nil, trace.Wrap(err)
	}

	globalNotification.Spec.Notification.Spec.Created = timestamppb.New(s.clock.Now())

	created, err := s.globalNotificationService.CreateResource(ctx, globalNotification)
	return created, trace.Wrap(err)
}

// UpsertGlobalNotification upserts a global notification resource that has already had its contents validated and its defaults such as the generated UUID, created date, and expiry date set.
func (s *NotificationsService) UpsertGlobalNotification(ctx context.Context, globalNotification *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, error) {
	if err := services.ValidateGlobalNotification(globalNotification); err != nil {
		return nil, trace.Wrap(err)
	}

	// Precautionary check in case of accidental misuse.
	if globalNotification.Spec.Notification.Spec.Id == "" {
		return nil, trace.BadParameter("notification id is missing. Did you mean to use CreateGlobalNotification?")
	}

	created, err := s.globalNotificationService.UpsertResource(ctx, globalNotification)
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

	// Verify that the notification this state is for exists.
	notifServiceWithPrefix := s.userNotificationService.WithPrefix(username)
	if _, err := notifServiceWithPrefix.GetResource(ctx, state.Spec.NotificationId); err != nil {
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
	serviceWithPrefix := s.userNotificationStateService.WithPrefix(username)

	upserted, err := serviceWithPrefix.UpsertResource(ctx, state)
	return upserted, trace.Wrap(err)
}

// DeleteUserNotificationState deletes a user notification state object.
func (s *NotificationsService) DeleteUserNotificationState(ctx context.Context, username string, notificationId string) error {
	// Append username prefix.
	serviceWithPrefix := s.userNotificationStateService.WithPrefix(username)

	err := serviceWithPrefix.DeleteResource(ctx, notificationId)
	return trace.Wrap(err)
}

// DeleteAllUserNotificationStatesForUser deletes all of a user's notification states.
func (s *NotificationsService) DeleteAllUserNotificationStatesForUser(ctx context.Context, username string) error {
	// Append username prefix.
	serviceWithPrefix := s.userNotificationStateService.WithPrefix(username)

	err := serviceWithPrefix.DeleteAllResources(ctx)
	return trace.Wrap(err)
}

// ListUserNotificationStates returns a page of a user's notification states.
func (s *NotificationsService) ListUserNotificationStates(ctx context.Context, username string, pageSize int, nextToken string) ([]*notificationsv1.UserNotificationState, string, error) {
	// Append username prefix.
	serviceWithPrefix := s.userNotificationStateService.WithPrefix(username)

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
func CheckAndSetExpiry(notification *notificationsv1.Notification, clock clockwork.Clock) error {
	// If the expiry hasn't been provided, set the default to 30 days from now.
	if notification.Metadata.Expires == nil {
		now := clock.Now()
		futureTime := now.Add(defaultExpiry)
		notification.Metadata.Expires = timestamppb.New(futureTime)
		return nil
	}

	// If the expiry has already been provided, ensure that it is not more than 90 days from now.
	// This is to prevent misuse as we don't want notifications existing for too long and accumulating in the backend.
	now := clock.Now()
	timeOfMaxExpiry := now.Add(maxExpiry)

	if (*notification.Metadata.Expires).AsTime().After(timeOfMaxExpiry) {
		return trace.BadParameter("notification expiry cannot be more than %d days from its creation", int(maxExpiry.Hours()/24))
	}

	return nil
}

const (
	notificationsGlobalPrefix       = "notifications/global"    // notifications/global/<notification id>
	notificationsUserSpecificPrefix = "notifications/user"      // notifications/user/<username>/<notification id>
	notificationsStatePrefix        = "notifications/states"    // notifications/states/<username>/<notification id>
	notificationsUserLastSeenPrefix = "notifications/last_seen" // notifications/last_seen/<username>

	defaultExpiry = 30 * 24 * time.Hour // The default expiry for a notification, 30 days.
	maxExpiry     = 90 * 24 * time.Hour // The maximum expiry for a notification, 90 days.
)
