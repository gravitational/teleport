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

package auth_test

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
)

func TestNotificationMatchers(t *testing.T) {
	t.Parallel()

	srv := mockAuth(t)

	roles := map[string]types.RoleSpecV6{
		// devs have access to nodes and databases, and the "root" and "ec2-user" logins
		"devs": {
			Allow: types.RoleConditions{
				Logins: []string{"root", "ec2-user"},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindNode, types.KindDatabase},
						Verbs:     services.RW(),
					},
				},
			},
		},
		// managers can review requests for the "intern" role
		"managers": {
			Allow: types.RoleConditions{
				Logins: []string{"user"},
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{"intern"},
				},
			},
		},
		// tokenadmins can read and write tokens
		"tokenadmins": {
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					{
						Resources: []string{types.KindToken},
						Verbs:     services.RW(),
					},
				},
			},
		},
	}
	for name, spec := range roles {
		role, err := types.NewRole(name, spec)
		require.NoError(t, err)

		_, err = srv.Auth().UpsertRole(t.Context(), role)
		require.NoError(t, err)
	}

	userRoles := map[string][]string{
		"bob":   []string{"devs"},
		"alice": []string{"managers"},
		"carol": []string{"devs", "tokenadmins"},
	}
	for username, roles := range userRoles {
		user, err := types.NewUser(username)
		require.NoError(t, err)
		user.SetRoles(roles)

		_, err = srv.Auth().UpsertUser(t.Context(), user)
		require.NoError(t, err)
	}

	// The number in each notification's description represents the order in which the
	// notification was created. For example, bob-3 will be the third notification created
	// for bob, and third from last in the list (since it's the third oldest).
	testNotifications := []struct {
		userNotification   *notificationsv1.Notification
		globalNotification *notificationsv1.GlobalNotification
	}{
		// Some notifications targeted at single users
		{userNotification: newUserNotification("bob", "bob-1")},
		{userNotification: newUserNotification("alice", "alice-1")},

		// A notification targeted at a specific role (managers)
		{
			globalNotification: newGlobalNotification("alice-2", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByRoles{
					ByRoles: &notificationsv1.ByRoles{
						Roles: []string{"managers"},
					},
				},
			}),
		},

		// A notification for everyone
		{
			globalNotification: newGlobalNotification("bob-2,alice-3,carol-1", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_All{All: true},
			}),
		},

		// A notification targeted to users with specific permissions (create tokens)
		{
			globalNotification: newGlobalNotification("carol-2", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								Rules: []types.Rule{
									{
										Resources: []string{types.KindToken},
										Verbs:     []string{types.VerbCreate},
									},
								},
							},
						},
					},
				},
			}),
		},

		// A notification targeted to users who can review certain access requests
		{
			globalNotification: newGlobalNotification("alice-4", &notificationsv1.GlobalNotificationSpec{
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

		// A notification for multiple users (by username)
		{
			globalNotification: newGlobalNotification("alice-5,bob-3", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByUsers{
					ByUsers: &notificationsv1.ByUsers{
						Users: []string{"alice", "bob"},
					},
				},
			}),
		},

		// Multiple matchers - logical OR
		{
			globalNotification: newGlobalNotification("alice-6,carol-3", &notificationsv1.GlobalNotificationSpec{
				MatchAllConditions: false, // logical OR
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								Logins: []string{"user"},
							},
							{
								Rules: []types.Rule{types.NewRule(types.KindToken, []string{types.VerbUpdate})},
							},
						},
					},
				},
			}),
		},

		// Multiple matchers - logical AND
		{
			globalNotification: newGlobalNotification("nomatches", &notificationsv1.GlobalNotificationSpec{
				MatchAllConditions: true, // logical AND
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								Logins: []string{"user"},
							},
							{
								Rules: []types.Rule{types.NewRule(types.KindToken, []string{types.VerbUpdate})},
							},
						},
					},
				},
			}),
		},

		// A notification for everyone but alice.
		{
			globalNotification: newGlobalNotification("bob-4,carol-4", &notificationsv1.GlobalNotificationSpec{
				Matcher:      &notificationsv1.GlobalNotificationSpec_All{All: true},
				ExcludeUsers: []string{"alice"},
			}),
		},

		// Multiple logins in a single rule. The notification goes to all users who
		// have at least one of the logins mentioned here.
		{
			globalNotification: newGlobalNotification("alice-7", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
					ByPermissions: &notificationsv1.ByPermissions{
						RoleConditions: []*types.RoleConditions{
							{
								Logins: []string{"user", "fakeuser1", "fakeuser2"},
							},
						},
					},
				},
			}),
		},
	}

	synctest.Test(t, func(t *testing.T) {
		for _, n := range testNotifications {
			if n.globalNotification != nil {
				_, err := srv.Auth().CreateGlobalNotification(t.Context(), n.globalNotification)
				require.NoError(t, err)
			}
			if n.userNotification != nil {
				_, err := srv.Auth().CreateUserNotification(t.Context(), n.userNotification)
				require.NoError(t, err)
			}

			// The "name" of the notification resource is a UUIDv7 based on the creation time.
			// Sleep between each upsert so that the notifications all have different timestamps.
			time.Sleep(1 * time.Minute)
		}
	})

	for _, test := range []struct {
		username string
		count    int
	}{
		{"bob", 4},
		{"alice", 7},
		{"carol", 4},
	} {
		t.Run(test.username, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(test.username))
			require.NoError(t, err)
			t.Cleanup(func() { client.Close() })

			resp, err := client.ListNotifications(t.Context(), &notificationsv1.ListNotificationsRequest{PageSize: 100})
			require.NoError(t, err)

			titles := notificationTitles(resp.Notifications)
			assert.Len(t, titles, test.count)

			for _, title := range titles {
				assert.Contains(t, title, test.username)
			}
		})
	}
}

func TestNotificationStates(t *testing.T) {
	t.Parallel()

	srv := mockAuth(t)

	bob, _, err := authtest.CreateUserAndRole(srv.Auth(), "bob", []string{"bob"}, nil /* allowRules */)
	require.NoError(t, err)

	var created []*notificationsv1.Notification
	synctest.Test(t, func(t *testing.T) {
		for i := range 4 {
			n, err := srv.Auth().CreateUserNotification(t.Context(), newUserNotification(bob.GetName(), fmt.Sprintf("notification-%d", i)))
			require.NoError(t, err)

			created = append(created, n)

			// The "name" of the notification resource is a UUIDv7 based on the creation time.
			// Sleep between each upsert so that the notifications all have different timestamps.
			time.Sleep(10 * time.Minute)
		}
	})

	client, err := srv.NewClient(authtest.TestUser(bob.GetName()))
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	// Update bob's last seen timestamp, which should make the bob-0 notification go away.
	_, err = client.UpsertUserLastSeenNotification(t.Context(), "bob", &notificationsv1.UserLastSeenNotification{
		Status: &notificationsv1.UserLastSeenNotificationStatus{
			LastSeenTime: created[0].GetSpec().GetCreated(),
		},
	})
	require.NoError(t, err)

	// RBAC check - bob should not be able to set last seen time for other users.
	_, err = client.UpsertUserLastSeenNotification(t.Context(), "alice", &notificationsv1.UserLastSeenNotification{
		Status: &notificationsv1.UserLastSeenNotificationStatus{
			LastSeenTime: created[0].GetSpec().GetCreated(),
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// Mark notification-1 as dismissed, so it stops showing up.
	_, err = client.UpsertUserNotificationState(t.Context(), "bob", &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: created[1].GetMetadata().GetName(),
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED,
		},
	})
	require.NoError(t, err)

	// Mark notification-2 as clicked. It will still show up.
	_, err = client.UpsertUserNotificationState(t.Context(), "bob", &notificationsv1.UserNotificationState{
		Spec: &notificationsv1.UserNotificationStateSpec{
			NotificationId: created[2].GetMetadata().GetName(),
		},
		Status: &notificationsv1.UserNotificationStateStatus{
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
		},
	})
	require.NoError(t, err)

	resp, err := client.ListNotifications(t.Context(), &notificationsv1.ListNotificationsRequest{PageSize: 10})
	require.NoError(t, err)

	require.Len(t, resp.Notifications, 3)
	require.Equal(t, []string{"notification-3", "notification-2", "notification-0"}, notificationTitles(resp.Notifications))

	require.Empty(t, resp.Notifications[0].GetMetadata().GetLabels()[types.NotificationClickedLabel])
	require.Equal(t, "true", resp.Notifications[1].GetMetadata().GetLabels()[types.NotificationClickedLabel])
	require.Empty(t, resp.Notifications[2].GetMetadata().GetLabels()[types.NotificationClickedLabel])
}

func TestNotificationPagination(t *testing.T) {
	t.Parallel()

	srv := mockAuth(t)

	bob, _, err := authtest.CreateUserAndRole(srv.Auth(), "bob", []string{"bob"}, nil /* allowRules */)
	require.NoError(t, err)

	synctest.Test(t, func(t *testing.T) {
		for i := range 10 {
			_, err = srv.Auth().CreateUserNotification(t.Context(), newUserNotification(bob.GetName(), fmt.Sprintf("notification-%d", i)))
			require.NoError(t, err)

			// The "name" of the notification resource is a UUIDv7 based on the creation time.
			// Sleep between each upsert so that the notifications all have different timestamps.
			time.Sleep(1 * time.Minute)
		}
	})

	client, err := srv.NewClient(authtest.TestUser(bob.GetName()))
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	// List the notifications in a few batches with different page sizes.
	var all []*notificationsv1.Notification
	var token string

	resp, err := client.ListNotifications(t.Context(), &notificationsv1.ListNotificationsRequest{PageSize: 3, PageToken: token})
	require.NoError(t, err)
	require.Len(t, resp.Notifications, 3)
	all = append(all, resp.Notifications...)
	token = resp.NextPageToken

	resp, err = client.ListNotifications(t.Context(), &notificationsv1.ListNotificationsRequest{PageSize: 4, PageToken: token})
	require.NoError(t, err)
	require.Len(t, resp.Notifications, 4)
	all = append(all, resp.Notifications...)
	token = resp.NextPageToken

	resp, err = client.ListNotifications(t.Context(), &notificationsv1.ListNotificationsRequest{PageSize: 1, PageToken: token})
	require.NoError(t, err)
	require.Len(t, resp.Notifications, 1)
	all = append(all, resp.Notifications...)
	token = resp.NextPageToken

	resp, err = client.ListNotifications(t.Context(), &notificationsv1.ListNotificationsRequest{PageSize: 10, PageToken: token})
	require.NoError(t, err)
	require.Len(t, resp.Notifications, 2) // only 2 remaining
	all = append(all, resp.Notifications...)

	require.Len(t, all, 10)

	var lastUUID string
	for i, notification := range all {
		// Listing returns the newest notifications first.
		want := fmt.Sprintf("notification-%d", 10-1-i)
		got := notification.GetMetadata().GetLabels()[types.NotificationTitleLabel]
		require.Equal(t, want, got)

		// We should be going "back in time" as we enumerate the list.
		if lastUUID != "" {
			require.Less(t, notification.GetMetadata().GetName(), lastUUID, notificationNames(all))
		}
		lastUUID = notification.GetMetadata().GetName()
	}
}

func newUserNotification(username string, title string) *notificationsv1.Notification {
	return &notificationsv1.Notification{
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
}

func newGlobalNotification(title string, spec *notificationsv1.GlobalNotificationSpec) *notificationsv1.GlobalNotification {
	spec.Notification = &notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec:    &notificationsv1.NotificationSpec{},
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{
				types.NotificationTitleLabel: title,
			},
		},
	}

	return &notificationsv1.GlobalNotification{Spec: spec}
}

func mockAuth(t *testing.T) *authtest.TLSServer {
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:          t.TempDir(),
		CacheEnabled: true,
		AuditLog:     &eventstest.MockAuditLog{Emitter: new(eventstest.MockRecorderEmitter)},
	})
	require.NoError(t, err)

	srv, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	return srv
}

func notificationTitles(notifications []*notificationsv1.Notification) []string {
	var titles []string
	for _, n := range notifications {
		title := n.GetMetadata().GetLabels()[types.NotificationTitleLabel]
		titles = append(titles, title)
	}
	return titles
}

func notificationNames(notifications []*notificationsv1.Notification) []string {
	var names []string
	for _, n := range notifications {
		name := n.GetMetadata().GetName()
		names = append(names, name)
	}
	return names
}
