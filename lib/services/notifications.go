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

package services

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// Notifications defines an interface for managing notifications.
type Notifications interface {
	ListUserNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.Notification, string, error)
	ListGlobalNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.GlobalNotification, string, error)
	DeleteAllUserNotifications(ctx context.Context) error
	DeleteAllGlobalNotifications(ctx context.Context) error
	CreateUserNotification(ctx context.Context, notification *notificationsv1.Notification) (*notificationsv1.Notification, error)
	UpsertUserNotification(ctx context.Context, notification *notificationsv1.Notification) (*notificationsv1.Notification, error)
	DeleteUserNotification(ctx context.Context, username string, notificationId string) error
	DeleteAllUserNotificationsForUser(ctx context.Context, username string) error
	CreateGlobalNotification(ctx context.Context, globalNotification *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, error)
	UpsertGlobalNotification(ctx context.Context, globalNotification *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, error)
	DeleteGlobalNotification(ctx context.Context, notificationId string) error
	UpsertUserNotificationState(ctx context.Context, username string, state *notificationsv1.UserNotificationState) (*notificationsv1.UserNotificationState, error)
	DeleteUserNotificationState(ctx context.Context, username string, notificationId string) error
	DeleteAllUserNotificationStatesForUser(ctx context.Context, username string) error
	ListUserNotificationStates(ctx context.Context, username string, pageSize int, nextToken string) ([]*notificationsv1.UserNotificationState, string, error)
	UpsertUserLastSeenNotification(ctx context.Context, username string, ulsn *notificationsv1.UserLastSeenNotification) (*notificationsv1.UserLastSeenNotification, error)
	GetUserLastSeenNotification(ctx context.Context, username string) (*notificationsv1.UserLastSeenNotification, error)
	DeleteUserLastSeenNotification(ctx context.Context, username string) error
}

// ValidateNotification verifies that the necessary fields are configured for a notification object.
func ValidateNotification(notification *notificationsv1.Notification) error {
	if notification.SubKind == "" {
		return trace.BadParameter("notification subkind is missing")
	}

	if notification.Spec == nil {
		return trace.BadParameter("notification spec is missing")
	}

	if notification.Metadata == nil {
		return trace.BadParameter("notification metadata is missing")
	}

	if notification.Metadata.Labels == nil {
		return trace.BadParameter("notification metadata labels are missing")
	}

	return nil
}

// MarshalNotification marshals a Notification resource to JSON.
func MarshalNotification(notification *notificationsv1.Notification, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateNotification(notification); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		notification = proto.Clone(notification).(*notificationsv1.Notification)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		notification.Metadata.Id = 0
		notification.Metadata.Revision = ""
	}
	data, err := utils.FastMarshal(notification)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalNotification unmarshals a Notification resource from JSON.
func UnmarshalNotification(data []byte, opts ...MarshalOption) (*notificationsv1.Notification, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing notification data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj notificationsv1.Notification
	if err = utils.FastUnmarshal(data, &obj); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Id is deprecated, but still needed.
		obj.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}

	return &obj, nil
}

// ValidateGlobalNotification verifies that the necessary fields are configured for a global notification object.
func ValidateGlobalNotification(globalNotification *notificationsv1.GlobalNotification) error {
	if globalNotification.Spec == nil {
		return trace.BadParameter("notification spec is missing")
	}

	if globalNotification.Spec.Matcher == nil {
		return trace.BadParameter("matcher is missing, a matcher is required for a global notification")
	}

	if err := ValidateNotification(globalNotification.Spec.Notification); err != nil {
		return trace.Wrap(err)
	}

	if globalNotification.Spec.Notification.Spec.Username != "" {
		return trace.BadParameter("a global notification cannot have a username")
	}

	return nil
}

// MarshalGlobalNotification marshals a GlobalNotification resource to JSON.
func MarshalGlobalNotification(globalNotification *notificationsv1.GlobalNotification, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateGlobalNotification(globalNotification); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		globalNotification = proto.Clone(globalNotification).(*notificationsv1.GlobalNotification)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		globalNotification.Metadata.Id = 0
		globalNotification.Metadata.Revision = ""
	}
	// We marshal with raw protojson here because utils.FastMarshal doesn't work with oneof.
	data, err := protojson.Marshal(globalNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalGlobalNotification unmarshals a GlobalNotification resource from JSON.
func UnmarshalGlobalNotification(data []byte, opts ...MarshalOption) (*notificationsv1.GlobalNotification, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing notification data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj notificationsv1.GlobalNotification
	// We unmarshal with raw protojson here because utils.FastUnmarshal doesn't work with oneof.
	if err = protojson.Unmarshal(data, &obj); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Id is deprecated, but still needed.
		obj.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &obj, nil
}

// ValidateUserNotificationState verifies that the necessary fields are configured for user notification state object.
func ValidateUserNotificationState(notificationState *notificationsv1.UserNotificationState) error {
	if notificationState.Spec.NotificationId == "" {
		return trace.BadParameter("notification id is missing")
	}

	if notificationState.Status == nil {
		return trace.BadParameter("notification state status is missing")
	}

	return nil
}

// MarshalUserNotificationState marshals a UserNotificationState resource to JSON.
func MarshalUserNotificationState(notificationState *notificationsv1.UserNotificationState, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateUserNotificationState(notificationState); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		notificationState = proto.Clone(notificationState).(*notificationsv1.UserNotificationState)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		notificationState.Metadata.Id = 0
		notificationState.Metadata.Revision = ""
	}
	data, err := utils.FastMarshal(notificationState)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalUserNotificationState unmarshals a UserNotificationState resource from JSON.
func UnmarshalUserNotificationState(data []byte, opts ...MarshalOption) (*notificationsv1.UserNotificationState, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing notification data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj notificationsv1.UserNotificationState
	if err = utils.FastUnmarshal(data, &obj); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Id is deprecated, but still needed.
		obj.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &obj, nil
}

// ValidateUserLastSeenNotification verifies that the necessary fields are configured for a user's last seen notification timestamp object.
func ValidateUserLastSeenNotification(lastSeenNotification *notificationsv1.UserLastSeenNotification) error {
	if lastSeenNotification.Status.LastSeenTime == nil {
		return trace.BadParameter("last seen time is missing")
	}

	return nil
}

// MarshalUserLastSeenNotification marshals a UserLastSeenNotification resource to JSON.
func MarshalUserLastSeenNotification(userLastSeenNotification *notificationsv1.UserLastSeenNotification, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateUserLastSeenNotification(userLastSeenNotification); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		userLastSeenNotification = proto.Clone(userLastSeenNotification).(*notificationsv1.UserLastSeenNotification)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		userLastSeenNotification.Metadata.Id = 0
		userLastSeenNotification.Metadata.Revision = ""
	}
	data, err := utils.FastMarshal(userLastSeenNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalUserLastSeenNotification unmarshals a UserLastSeenNotification resource from JSON.
func UnmarshalUserLastSeenNotification(data []byte, opts ...MarshalOption) (*notificationsv1.UserLastSeenNotification, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing notification data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj notificationsv1.UserLastSeenNotification
	if err = utils.FastUnmarshal(data, &obj); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Id is deprecated, but still needed.
		obj.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &obj, nil
}
