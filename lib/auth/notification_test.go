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
	"testing"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type notificationTestPack struct {
	tlsServer *TestTLSServer
	roles     map[string]types.RoleSpecV6
	users     map[string][]string
}

func TestNotifications(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err, "%s", trace.DebugReport(err))
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	tlsServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

	roles := map[string]types.RoleSpecV6{
		// auditors have access to nodes and databases, and the "auditor" login
		"auditors": {
			Allow: types.RoleConditions{
				Logins: []string{"auditor"},
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
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{"intern"},
				},
			},
		},
	}
	for roleName, roleSpec := range roles {
		role, err := types.NewRole(roleName, roleSpec)
		require.NoError(t, err)

		_, err = tlsServer.Auth().UpsertRole(ctx, role)
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
		_, err = tlsServer.Auth().UpsertUser(ctx, user)
		require.NoError(t, err)
	}

	// Describes a set of mock notifications to be created.
	testNotifications := []struct {
		userNotification   *notificationsv1.Notification
		globalNotification *notificationsv1.GlobalNotification
	}{
		{
			userNotification: newUserNotification(t),
		},
	}

	var auditorNotificationIds []string

}

func newUserNotification(t *testing.T, description string) *notificationsv1.Notification {
	t.Helper()

	notification := notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec:    &notificationsv1.NotificationSpec{},
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
