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

package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestNotifications(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fakeClock := clockwork.NewFakeClock()

	authServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:          t.TempDir(),
		Clock:        fakeClock,
		CacheEnabled: true,
	})
	require.NoError(t, err)

	srv, err := authServer.NewTestTLSServer()

	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	roles := map[string]types.RoleSpecV6{
		// auditors have access to nodes and databases, and the "auditor" and "user" logins
		"auditors": {
			Allow: types.RoleConditions{
				Logins: []string{"auditor", "user"},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindNode, types.KindDatabase},
						Verbs:     services.RW(),
					},
				},
			},
		},
		// managers have access to review requests for the "intern" role
		"managers": {
			Allow: types.RoleConditions{
				Logins: []string{"user"},
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{"intern"},
				},
			},
		},
	}
	for roleName, roleSpec := range roles {
		role, err := types.NewRole(roleName, roleSpec)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertRole(ctx, role)
		require.NoError(t, err)
	}

	auditorUsername := "auditor"
	managerUsername := "manager"

	userRolesMap := map[string][]string{
		auditorUsername: {"auditors"},
		managerUsername: {"managers"},
	}

	// Create the users with their roles.
	for username, roles := range userRolesMap {
		user, err := types.NewUser(username)
		require.NoError(t, err)
		user.SetRoles(roles)
		_, err = srv.Auth().UpsertUser(ctx, user)
		require.NoError(t, err)
	}

	// Describes a set of mock notifications to be created.
	// The number in each notification's description represents the creation order of the notifications.
	// eg. "auditor-3" will be the third notification created for auditor, and the third from last in the list (since it's the third oldest),
	testNotifications := []struct {
		userNotification   *notificationsv1.Notification
		globalNotification *notificationsv1.GlobalNotification
	}{
		{
			userNotification: newUserNotification(t, auditorUsername, "auditor-1"),
		},
		{
			userNotification: newUserNotification(t, auditorUsername, "auditor-2"),
		},
		{
			// Matcher matches by the role "auditors"
			globalNotification: newGlobalNotification(t, "auditor-3", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByRoles{
					ByRoles: &notificationsv1.ByRoles{
						Roles: []string{userRolesMap[auditorUsername][0]},
					},
				},
			}),
		},
		{
			// Matcher matches by the role "managers"
			globalNotification: newGlobalNotification(t, "manager-1", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByRoles{
					ByRoles: &notificationsv1.ByRoles{
						Roles: []string{userRolesMap[managerUsername][0]},
					},
				},
			}),
		},
		{
			userNotification: newUserNotification(t, auditorUsername, "auditor-4"),
		},
		{
			// Matcher matches all, both users should see this.
			globalNotification: newGlobalNotification(t, "auditor-5,manager-2", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_All{
					All: true,
				},
			}),
		},
		{
			userNotification: newUserNotification(t, managerUsername, "manager-3"),
		},
		{
			// Matcher matches by read & write permission on nodes.
			globalNotification: newGlobalNotification(t, "auditor-6", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								Rules: []types.Rule{
									{
										Resources: []string{types.KindNode},
										Verbs:     services.RW(),
									},
								},
							},
						},
					},
				},
			}),
		},
		{
			// Matcher matches by the logins "auditor" and "user". Auditor has both of them, but manager only has "user", since we set MatchAllConditions to true, only auditor
			// should get this notification.
			globalNotification: newGlobalNotification(t, "auditor-7", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								Logins: []string{"auditor"},
							},
							{
								Logins: []string{"user"},
							},
						},
					},
				},
				MatchAllConditions: true,
			}),
		},
		{
			// Matcher matches by permission to review access requests for "intern"
			globalNotification: newGlobalNotification(t, "manager-4", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								ReviewRequests: &types.AccessReviewConditions{
									Roles: []string{"intern"},
								},
							},
						},
					},
				},
			}),
		},
		{
			userNotification: newUserNotification(t, managerUsername, "manager-5"),
		},
		{
			// Matcher matches by the logins "auditor" and "user". Both of them have "user" and MatchAllConditions is false, so both should get this notification.
			globalNotification: newGlobalNotification(t, "auditor-8,manager-6", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								Logins: []string{"auditor"},
							},
							{
								Logins: []string{"user"},
							},
						},
					},
				},
			}),
		},
		{
			userNotification: &notificationsv1.Notification{
				SubKind: "test-subkind",
				Spec: &notificationsv1.NotificationSpec{
					Username: managerUsername,
				},
				Metadata: &headerv1.Metadata{
					Labels: map[string]string{
						types.NotificationTitleLabel: "manager-7-expires",
					},
					// Expires in 15 minutes.
					Expires: timestamppb.New(fakeClock.Now().Add(15 * time.Minute)),
				},
			},
		},
		{
			globalNotification: &notificationsv1.GlobalNotification{
				Spec: &notificationsv1.GlobalNotificationSpec{
					Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
						ByPermissions: &notificationsv1.ByPermissions{
							RoleConditions: []*types.RoleConditions{
								{
									ReviewRequests: &types.AccessReviewConditions{
										Roles: []string{"intern"},
									},
								},
							},
						},
					},
					Notification: &notificationsv1.Notification{
						SubKind: "test-subkind",
						Spec:    &notificationsv1.NotificationSpec{},
						Metadata: &headerv1.Metadata{
							Labels: map[string]string{
								types.NotificationTitleLabel: "manager-8-expires",
							},
							// Expires in 10 minutes.
							Expires: timestamppb.New(fakeClock.Now().Add(10 * time.Minute)),
						},
					},
				},
			},
		},
		{
			// Matcher matches by usernames.
			globalNotification: newGlobalNotification(t, "auditor-9,manager-9", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByUsers{
					ByUsers: &notificationsv1.ByUsers{
						Users: []string{auditorUsername, managerUsername},
					},
				},
			}),
		},
	}

	notificationIdMap := map[string]string{}

	// Create the notifications.
	for _, n := range testNotifications {
		// We add a small delay to ensure that the timestamps in the generated UUID's are all different and in the correct order.
		// This is to prevent flakiness caused by the notifications being created in such quick succession that the timestamps are the same.
		fakeClock.Advance(50 * time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		// Create the notification in the backend.
		if n.globalNotification != nil {
			created, err := srv.Auth().CreateGlobalNotification(ctx, n.globalNotification)
			notificationIdMap[created.GetSpec().GetNotification().GetMetadata().GetLabels()[types.NotificationTitleLabel]] = created.GetMetadata().GetName()
			require.NoError(t, err)
			continue
		}
		created, err := srv.Auth().CreateUserNotification(ctx, n.userNotification)
		notificationIdMap[created.GetMetadata().GetLabels()[types.NotificationTitleLabel]] = created.GetMetadata().GetName()
		require.NoError(t, err)
	}

	// Test fetching notifications for auditor.
	auditorClient, err := srv.NewClient(TestUser(auditorUsername))
	require.NoError(t, err)
	defer auditorClient.Close()

	auditorExpectedNotifications := []string{"auditor-9,manager-9", "auditor-8,manager-6", "auditor-7", "auditor-6", "auditor-5,manager-2", "auditor-4", "auditor-3", "auditor-2", "auditor-1"}

	var finalOut []*notificationsv1.Notification

	// Upsert auditor's last seen timestamp.
	lastSeenTimestamp := timestamppb.New(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	_, err = auditorClient.UpsertUserLastSeenNotification(ctx, auditorUsername, &notificationsv1.UserLastSeenNotification{
		Status: &notificationsv1.UserLastSeenNotificationStatus{
			LastSeenTime: lastSeenTimestamp,
		},
	})
	require.NoError(t, err)

	// Fetch a page of 3 notifications.
	resp, err := auditorClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 3,
	})
	require.NoError(t, err)
	require.Equal(t, auditorExpectedNotifications[:3], notificationsToTitlesList(t, resp.Notifications))
	finalOut = append(finalOut, resp.Notifications...)

	// Verify that the nextKeys are correct.
	expectedUserNotifsNextKey := notificationIdMap["auditor-4"]
	expectedGlobalNotifsNextKey := notificationIdMap["auditor-6"]
	expectedNextKeys := fmt.Sprintf("%s,%s",
		expectedUserNotifsNextKey,
		expectedGlobalNotifsNextKey) // "<auditor-4 notification id>,<auditor-6 notification id>"

	require.Equal(t, expectedNextKeys, resp.NextPageToken)
	require.Equal(t, lastSeenTimestamp.GetSeconds(), resp.UserLastSeenNotificationTimestamp.GetSeconds())

	// Fetch the next 4 notifications, starting from the previously received startKeys.
	// After this fetch, there should be no more global notifications for auditor, so the next page token
	// for global notifications should be "".
	resp, err = auditorClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize:  4,
		PageToken: resp.NextPageToken,
	})
	require.NoError(t, err)
	expectedNextKeys = fmt.Sprintf("%s,", notificationIdMap["auditor-2"])

	require.Equal(t, expectedNextKeys, resp.NextPageToken)
	finalOut = append(finalOut, resp.Notifications...)

	// Fetch a page of 1 notification, starting from the previously received startKeys.
	resp, err = auditorClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize:  1,
		PageToken: resp.NextPageToken,
	})
	require.NoError(t, err)
	expectedNextKeys = fmt.Sprintf("%s,", notificationIdMap["auditor-1"])
	require.Equal(t, expectedNextKeys, resp.NextPageToken)
	finalOut = append(finalOut, resp.Notifications...)

	// Fetch the rest of the notifications, starting from the previously received startKeys.
	resp, err = auditorClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize:  10,
		PageToken: resp.NextPageToken,
	})
	require.NoError(t, err)
	finalOut = append(finalOut, resp.Notifications...)
	// Verify that all the notifications are in the list and in correct order.
	require.Equal(t, auditorExpectedNotifications, notificationsToTitlesList(t, finalOut))
	// Verify that we've reached the end of both lists.
	require.Empty(t, resp.NextPageToken)

	// Mark "auditor-2" and "auditor-5,manager-2" as dismissed.
	_, err = auditorClient.UpsertUserNotificationState(ctx, auditorUsername, &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: notificationIdMap["auditor-2"],
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED,
		},
	})
	require.NoError(t, err)
	_, err = auditorClient.UpsertUserNotificationState(ctx, auditorUsername, &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: notificationIdMap["auditor-5,manager-2"],
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED,
		},
	})
	require.NoError(t, err)

	// Fetch notifications again and verify that the dismissed notifications are not returned.
	resp, err = auditorClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 10,
	})
	auditorExpectedNotifsAfterDismissal := []string{"auditor-9,manager-9", "auditor-8,manager-6", "auditor-7", "auditor-6", "auditor-4", "auditor-3", "auditor-1"}
	require.NoError(t, err)
	require.Equal(t, auditorExpectedNotifsAfterDismissal, notificationsToTitlesList(t, resp.Notifications))

	// Test fetching notifications for manager.
	managerClient, err := srv.NewClient(TestUser(managerUsername))
	require.NoError(t, err)
	defer managerClient.Close()

	managerExpectedNotifications := []string{"auditor-9,manager-9", "manager-8-expires", "manager-7-expires", "auditor-8,manager-6", "manager-5", "manager-4", "manager-3", "auditor-5,manager-2", "manager-1"}

	resp, err = managerClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 10,
	})
	require.NoError(t, err)

	require.Equal(t, managerExpectedNotifications, notificationsToTitlesList(t, resp.Notifications))
	// Verify that we've reached the end of both lists.
	require.Empty(t, resp.NextPageToken)

	// Mark "manager-8-expires" as clicked.
	_, err = managerClient.UpsertUserNotificationState(ctx, managerUsername, &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: notificationIdMap["manager-8-expires"],
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
		},
	})
	require.NoError(t, err)

	// Fetch notifications again and expect it to have the clicked label.
	resp, err = managerClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 10,
	})
	require.NoError(t, err)

	clickedNotification := resp.Notifications[1] // "manager-8-expires" is the second item in the list
	clickedLabelValue := clickedNotification.GetMetadata().GetLabels()[types.NotificationClickedLabel]
	require.Equal(t, "true", clickedLabelValue)

	// Advance 11 minutes.
	fakeClock.Advance(11 * time.Minute)

	// Verify that notification "manager-8-expires" is now no longer returned.
	resp, err = managerClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{})
	require.NoError(t, err)
	require.NotContains(t, notificationsToTitlesList(t, resp.Notifications), "manager-8-expires")

	// Advance 16 minutes.
	fakeClock.Advance(16 * time.Minute)

	// Verify that notification "manager-7-expires" is now no longer returned either.
	resp, err = managerClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{})
	require.NoError(t, err)
	require.NotContains(t, notificationsToTitlesList(t, resp.Notifications), "manager-7-expires")

	// Verify that manager can't upsert a notification state for auditor
	_, err = managerClient.UpsertUserNotificationState(ctx, auditorUsername, &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: notificationIdMap["auditor-7"],
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "got error %T, expected an access denied error due to manager trying to upsert a notification state for a different user", err)

	// Verify that manager can't uspert a user last seen notification for auditor.
	_, err = managerClient.UpsertUserLastSeenNotification(ctx, auditorUsername, &notificationsv1.UserLastSeenNotification{
		Status: &notificationsv1.UserLastSeenNotificationStatus{
			LastSeenTime: lastSeenTimestamp,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "got error %T, expected an access denied error due to manager trying to upsert a last seen notification timestamp for a different user", err)

	// Verify that users can't list a global notification they are explicitly excluded from.

	// Create a global notification that matches all users with an exclusion for the manager.
	_, err = srv.Auth().CreateGlobalNotification(ctx, newGlobalNotification(t, "all-except-manager", &notificationsv1.GlobalNotificationSpec{
		Matcher: &notificationsv1.GlobalNotificationSpec_All{
			All: true,
		},
		ExcludeUsers: []string{managerUsername},
	}))
	require.NoError(t, err)

	// Verify that the manager doesn't see the new notification.
	resp, err = managerClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 10,
	})
	require.NoError(t, err)
	require.NotEqual(t, "all-except-manager", resp.Notifications[0].GetMetadata().GetLabels()[types.NotificationTitleLabel])

	// Verify that the auditor can see the new notification.
	resp, err = auditorClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Equal(t, "all-except-manager", resp.Notifications[0].GetMetadata().GetLabels()[types.NotificationTitleLabel])

}

func newUserNotification(t *testing.T, username string, title string) *notificationsv1.Notification {
	t.Helper()

	notification := notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec: &notificationsv1.NotificationSpec{
			Username: username,
		},
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{
				types.NotificationTitleLabel: title,
			},
		},
	}

	return &notification
}

func newGlobalNotification(t *testing.T, title string, spec *notificationsv1.GlobalNotificationSpec) *notificationsv1.GlobalNotification {
	t.Helper()

	spec.Notification = &notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec:    &notificationsv1.NotificationSpec{},
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{
				types.NotificationTitleLabel: title,
			},
		},
	}

	notification := notificationsv1.GlobalNotification{
		Spec: spec,
	}

	return &notification
}

// notificationsToTitlesList accepts a list of notifications notifications and returns a slice of strings containing their titles in order, this is used to compare against
// the expected outputs.
func notificationsToTitlesList(t *testing.T, notifications []*notificationsv1.Notification) []string {
	t.Helper()
	var descriptions []string

	for _, notif := range notifications {
		description := notif.GetMetadata().GetLabels()[types.NotificationTitleLabel]
		descriptions = append(descriptions, description)
	}

	return descriptions
}
