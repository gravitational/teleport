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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestMarshalNotificationRoundTrip tests the marshaling and unmarshaling functions for Notification objects.
func TestMarshalNotificationRoundTrip(t *testing.T) {
	notification := notificationsv1.Notification_builder{
		Kind:    types.KindNotification,
		Version: types.V1,
		SubKind: "test-subkind",
		Spec:    &notificationsv1.NotificationSpec{},
		Metadata: headerv1.Metadata_builder{
			Name:   "test-notification-1",
			Labels: map[string]string{types.NotificationTitleLabel: "title-1"},
		}.Build(),
	}.Build()

	payload, err := MarshalNotification(notification)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalNotification(payload)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(notification, unmarshaled, protocmp.Transform()))
}

// TestMarshalGlobalNotificationRoundTrip tests the marshaling and unmarshaling functions for GlobalNotification objects.
func TestMarshalGlobalNotificationRoundTrip(t *testing.T) {
	notification := notificationsv1.GlobalNotification_builder{
		Kind:     types.KindGlobalNotification,
		Metadata: &headerv1.Metadata{},
		Version:  types.V1,
		Spec: notificationsv1.GlobalNotificationSpec_builder{
			All: proto.Bool(true),
			Notification: notificationsv1.Notification_builder{
				SubKind: "test-subkind",
				Spec:    &notificationsv1.NotificationSpec{},
				Metadata: headerv1.Metadata_builder{
					Name:   "test-notification-id",
					Labels: map[string]string{types.NotificationTitleLabel: "title-1"},
				}.Build(),
			}.Build(),
		}.Build(),
	}.Build()

	payload, err := MarshalGlobalNotification(notification)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalGlobalNotification(payload)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(notification, unmarshaled, protocmp.Transform()))
}

// TestUserNotificationStateRoundTrip tests the marshaling and unmarshaling functions for UserNotificationState objects.
func TestUserNotificationStateRoundTrip(t *testing.T) {
	userNotificationState := notificationsv1.UserNotificationState_builder{
		Metadata: &headerv1.Metadata{},
		Spec: notificationsv1.UserNotificationStateSpec_builder{
			NotificationId: "test-notification-1",
		}.Build(),
		Status: notificationsv1.UserNotificationStateStatus_builder{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
		}.Build(),
	}.Build()

	payload, err := MarshalUserNotificationState(userNotificationState)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalUserNotificationState(payload)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(userNotificationState, unmarshaled, protocmp.Transform()))
}

// TestUserLastSeenNotificationRoundTrip tests the marshaling and unmarshaling functions for Notification objects.
func TestUserLastSeenNotificationStateRoundTrip(t *testing.T) {
	timestamp := timestamppb.New(time.UnixMilli(1708041600000)) // February 16, 2024 12:00:00 AM UTC
	userLastSeenNotification := notificationsv1.UserLastSeenNotification_builder{
		Metadata: &headerv1.Metadata{},
		Status: notificationsv1.UserLastSeenNotificationStatus_builder{
			LastSeenTime: timestamp,
		}.Build(),
	}.Build()

	payload, err := MarshalUserLastSeenNotification(userLastSeenNotification)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalUserLastSeenNotification(payload)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(userLastSeenNotification, unmarshaled, protocmp.Transform()))
}
