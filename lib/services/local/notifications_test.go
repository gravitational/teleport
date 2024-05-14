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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestUserNotificationCRUD tests backend operations for user-specific notification resources.
func TestUserNotificationCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewNotificationsService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	testUsername := "test-username"

	// Create a couple notifications.
	userNotification1 := newUserNotification(t, testUsername, "test-notification-1")
	userNotification2 := newUserNotification(t, testUsername, "test-notification-2")

	// Create notifications.
	notification, err := service.CreateUserNotification(ctx, userNotification1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(userNotification1, notification, protocmp.Transform()))
	notification1Id := notification.Spec.Id
	// Prevent flakiness caused by notifications being created too close one after the other, which causes their UUID timestamps to be the same
	// and the lexicographical ordering to possibly be wrong as it then relies on the random section of the UUID.
	time.Sleep(250 * time.Millisecond)
	notification, err = service.CreateUserNotification(ctx, userNotification2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(userNotification2, notification, protocmp.Transform()))
	notification2Id := notification.Spec.Id

	// Test deleting a notification.
	err = service.DeleteUserNotification(ctx, testUsername, notification1Id)
	require.NoError(t, err)
	// Since we don't have any Get or List method for user-specific notifications specifically, we will assert that it was deleted
	// by attempting to delete it again and expecting a "not found" error.
	err = service.DeleteUserNotification(ctx, testUsername, notification1Id)
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to notification test-notification-1 not existing", err)

	// Test deleting a notification that doesn't exist.
	err = service.DeleteUserNotification(ctx, testUsername, "invalid-id")
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to notification invalid-id not existing", err)

	// Test deleting all of a user's user-specific notifications.
	// Upsert userNotification1 again.
	// We reset it to the mock first because the previous CreateUserNotification will have mutated it and populated the `Created` field which should be empty.
	userNotification1 = newUserNotification(t, testUsername, "test-notification-1")
	_, err = service.CreateUserNotification(ctx, userNotification1)
	require.NoError(t, err)
	notification1Id = notification.Spec.Id
	err = service.DeleteAllUserNotificationsForUser(ctx, testUsername)
	require.NoError(t, err)
	// Verify that the notifications don't exist anymore by attempting to delete them.
	err = service.DeleteUserNotification(ctx, testUsername, notification1Id)
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to notification test-notification-1 not existing", err)
	err = service.DeleteUserNotification(ctx, testUsername, notification2Id)
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to notification test-notification-2 not existing", err)

}

// TestGlobalNotificationCRUD tests backend operations for global notification resources.
func TestGlobalNotificationCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewNotificationsService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	// Create a couple notifications.
	globalNotification1 := newGlobalNotification(t, "test-notification-1")
	globalNotification2 := newGlobalNotification(t, "test-notification-2")
	globalNotificationNoMatcher := &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Notification: &notificationsv1.Notification{
				SubKind: "test-subkind",
				Spec:    &notificationsv1.NotificationSpec{},
				Metadata: &headerv1.Metadata{
					Description: "Test Description",
				},
			},
		},
	}
	globalNotificationLateExpiry := &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_All{
				All: true,
			},
			Notification: &notificationsv1.Notification{
				SubKind: "test-subkind",
				Spec:    &notificationsv1.NotificationSpec{},
				Metadata: &headerv1.Metadata{
					Description: "Test Description",
					Labels:      map[string]string{"description": "notification-late-expiry"},
					// Set the expiry to 91 days from now, which is past the 90 day expiry limit.
					Expires: timestamppb.New(clock.Now().AddDate(0, 0, 91)),
				},
			},
		},
	}

	// Create notifications.
	notification, err := service.CreateGlobalNotification(ctx, globalNotification1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(globalNotification1, notification, protocmp.Transform()))
	globalNotification1Id := notification.Spec.Notification.Spec.Id
	notification, err = service.CreateGlobalNotification(ctx, globalNotification2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(globalNotification2, notification, protocmp.Transform()))
	// Expect error due to having no matcher.
	_, err = service.CreateGlobalNotification(ctx, globalNotificationNoMatcher)
	require.True(t, trace.IsBadParameter(err), "got error %T, expected a bad parameter error due to notification-no-matcher having no matcher", err)
	// Expect error due to expiry date being more than 90 days from now.
	_, err = service.CreateGlobalNotification(ctx, globalNotificationLateExpiry)
	require.True(t, trace.IsBadParameter(err), "got error %T, expected a bad parameter error due to notification-late-expiry having an expiry date more than 90 days later", err)

	// Test deleting a notification.
	err = service.DeleteGlobalNotification(ctx, globalNotification1Id)
	require.NoError(t, err)
	// Test deleting a notification that doesn't exist.
	err = service.DeleteGlobalNotification(ctx, "invalid-id")
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to notification invalid-id not existing", err)
}

// TestUserNotificationStateCRUD tests backend operations for user-specific notification resources.
func TestUserNotificationStateCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewNotificationsService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	testUsername := "test-username"

	// Create a and upsert the notifications that these states will be for.
	userNotification1 := newUserNotification(t, testUsername, "test-notification-1")
	userNotification2 := newUserNotification(t, testUsername, "test-notification-2")
	notification, err := service.CreateUserNotification(ctx, userNotification1)
	require.NoError(t, err)
	notification1Id := notification.Spec.Id
	// Prevent flakiness caused by notifications being created too close one after the other, which causes their UUID timestamps to be the same
	// and the lexicographical ordering to possibly be wrong as it then relies on the random section of the UUID.
	time.Sleep(250 * time.Millisecond)
	notification, err = service.CreateUserNotification(ctx, userNotification2)
	require.NoError(t, err)
	notification2Id := notification.Spec.Id

	userNotificationState1 := &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: notification1Id,
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
		},
	}

	// Duplicate of the above but with the state set to dismissed instead of clicked.
	userNotificationState1Dismissed := &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: notification1Id,
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED,
		},
	}

	userNotificationState2 := &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: notification2Id,
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
		},
	}

	// Initially we expect no user notification states.
	out, nextToken, err := service.ListUserNotificationStates(ctx, testUsername, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Upsert notification states.
	notificationState, err := service.UpsertUserNotificationState(ctx, testUsername, userNotificationState1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(userNotificationState1, notificationState, protocmp.Transform()))
	notificationState, err = service.UpsertUserNotificationState(ctx, testUsername, userNotificationState2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(userNotificationState2, notificationState, protocmp.Transform()))

	// Fetch a paginated list of the user's notification states.
	paginatedOut := make([]*notificationsv1.UserNotificationState, 0, 2)
	for {
		out, nextToken, err = service.ListUserNotificationStates(ctx, testUsername, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "id", "revision"),
		protocmp.Transform(),
	}

	require.Len(t, paginatedOut, 2)
	// Verify that notification states returned are correct.
	require.Empty(t, cmp.Diff([]*notificationsv1.UserNotificationState{userNotificationState1, userNotificationState2}, paginatedOut, cmpOpts...))

	// Upsert a dismissed state with for the same notification id as userNotificationState1.
	notificationState, err = service.UpsertUserNotificationState(ctx, testUsername, userNotificationState1Dismissed)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(userNotificationState1Dismissed, notificationState, cmpOpts...))

	// Fetch the list again.
	paginatedOut = make([]*notificationsv1.UserNotificationState, 0, 2)
	for {
		out, nextToken, err = service.ListUserNotificationStates(ctx, testUsername, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Len(t, paginatedOut, 2)
	// Verify that notification id's and states are correct, userNotificationState1 should now have the dismissed state.
	require.Equal(t, userNotificationState1.Spec.NotificationId, paginatedOut[0].Spec.NotificationId)
	require.Equal(t, notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED, paginatedOut[0].Status.NotificationState)
	require.Equal(t, userNotificationState2.Spec.NotificationId, paginatedOut[1].Spec.NotificationId)
	require.Equal(t, notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED, paginatedOut[1].Status.NotificationState)

	// Test deleting a notification state.
	err = service.DeleteUserNotificationState(ctx, testUsername, notification1Id)
	require.NoError(t, err)
	// Test deleting a notification state that doesn't exist.
	err = service.DeleteUserNotificationState(ctx, testUsername, "invalid-id")
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to notification invalid-id not existing", err)

	// Fetch the list again.
	paginatedOut = make([]*notificationsv1.UserNotificationState, 0, 2)
	for {
		out, nextToken, err = service.ListUserNotificationStates(ctx, testUsername, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	// Verify that only userNotificationState2 remains.
	require.Len(t, paginatedOut, 1)
	require.Empty(t, cmp.Diff([]*notificationsv1.UserNotificationState{userNotificationState2}, paginatedOut, cmpOpts...))

	// Upsert userNotificationState1 again.
	_, err = service.UpsertUserNotificationState(ctx, testUsername, userNotificationState1)
	require.NoError(t, err)

	// Test deleting all notification states for the user.
	err = service.DeleteAllUserNotificationStatesForUser(ctx, testUsername)
	require.NoError(t, err)
	// Verify that the user now has no notification states.
	out, nextToken, err = service.ListUserNotificationStates(ctx, testUsername, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}

// TestUserLastSeenNotificationCRUD tests backend operations for user last seen notification resources.
func TestUserLastSeenNotificationCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewNotificationsService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	testUsername := "test-username"
	testTimestamp := timestamppb.New(time.UnixMilli(1708041600000)) // February 16, 2024 12:00:00 AM UTC

	userLastSeenNotification := &notificationsv1.UserLastSeenNotification{
		Status: &notificationsv1.UserLastSeenNotificationStatus{
			LastSeenTime: testTimestamp,
		},
	}

	// Initially we expect the user's last seen notification object to not exist.
	_, err = service.GetUserLastSeenNotification(ctx, testUsername)
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to user_last_seen_notification for test-username not existing", err)

	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "id", "revision"),
		protocmp.Transform(),
	}

	// Upsert user last seen notification.
	ulsn, err := service.UpsertUserLastSeenNotification(ctx, testUsername, userLastSeenNotification)
	require.Empty(t, cmp.Diff(userLastSeenNotification, ulsn, cmpOpts...))
	require.NoError(t, err)

	// The user's last seen notification object should now exist.
	out, err := service.GetUserLastSeenNotification(ctx, testUsername)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(userLastSeenNotification, out, cmpOpts...))

	// Test deleting a user last seen notification object.
	err = service.DeleteUserLastSeenNotification(ctx, testUsername)
	require.NoError(t, err)
	// Deleting a non-existent user last seen notification object should return an error.
	err = service.DeleteUserLastSeenNotification(ctx, "invalid-username")
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to user_last_seen_notification for invalid-username not existing", err)

	// Getting the user's last seen notification object should now fail again since we deleted it.
	_, err = service.GetUserLastSeenNotification(ctx, testUsername)
	require.True(t, trace.IsNotFound(err), "got error %T, expected a not found error due to user_last_seen_notification for test-username not existing", err)
}

func newUserNotification(t *testing.T, username string, description string) *notificationsv1.Notification {
	t.Helper()

	notification := notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec: &notificationsv1.NotificationSpec{
			Username: username,
		},
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{"description": description},
		},
	}

	return &notification
}

func newGlobalNotification(t *testing.T, description string) *notificationsv1.GlobalNotification {
	t.Helper()

	notification := notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_All{
				All: true,
			},
			Notification: &notificationsv1.Notification{
				SubKind: "test-subkind",
				Spec:    &notificationsv1.NotificationSpec{},
				Metadata: &headerv1.Metadata{
					Labels: map[string]string{"description": description},
				},
			},
		},
	}

	return &notification
}
