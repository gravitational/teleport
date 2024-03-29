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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestNotifications(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	authServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
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
	// eg. "auditor-3" will be the third notification created for auditor, and the third from last in the list (since it is the third oldest),
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
	}

	// Create the notifications.
	for _, n := range testNotifications {
		// We add a delay to ensure that the timestamps in the generated UUID's are all different and in the correct order.
		time.Sleep(50 * time.Millisecond)
		// Create the notification in the backend.
		if n.globalNotification != nil {
			_, err := srv.Auth().CreateGlobalNotification(ctx, n.globalNotification)
			require.NoError(t, err)
			continue
		}
		_, err := srv.Auth().CreateUserNotification(ctx, n.userNotification)
		require.NoError(t, err)
	}

	// auditorExpectedNotifications := []string{"auditor-8,manager-6", "auditor-7", "auditor-6", "auditor-5,manager-2", "auditor-4", "auditor-3", "auditor-2", "auditor-1"}
	// managerExpectedNotifications := []string{"auditor-8,manager-6", "manager-5", "manager-4", "manager-3", "auditor-5,manager-2", "manager-1"}

	auditorClient, err := srv.NewClient(TestUser(auditorUsername))
	require.NoError(t, err)
	defer auditorClient.Close()

	managerClient, err := srv.NewClient(TestUser(managerUsername))
	require.NoError(t, err)
	defer managerClient.Close()

	resp, err := auditorClient.ListNotifications(ctx, &notificationsv1.ListNotificationsRequest{
		PageSize: 10,
	})

	fmt.Printf("\n\nresponse:\n%v\n", resp)

	test := newGlobalNotification(t, "auditor-8,manager-6", &notificationsv1.GlobalNotificationSpec{
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
	})

	auditorClient.CreateGlobalNotification(ctx, test)

	require.Equal(t, 5, 10)
}

func newUserNotification(t *testing.T, username string, description string) *notificationsv1.Notification {
	t.Helper()

	notification := notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec: &notificationsv1.NotificationSpec{
			Username: username,
		},
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{
				types.NotificationTitleLabel:       "test-title",
				types.NotificationDescriptionLabel: description},
		},
	}

	return &notification
}

func newGlobalNotification(t *testing.T, description string, spec *notificationsv1.GlobalNotificationSpec) *notificationsv1.GlobalNotification {
	t.Helper()

	spec.Notification = &notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec:    &notificationsv1.NotificationSpec{},
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{
				types.NotificationTitleLabel:       "test-title",
				types.NotificationDescriptionLabel: description},
		},
	}

	notification := notificationsv1.GlobalNotification{
		Spec: spec,
	}

	return &notification
}
