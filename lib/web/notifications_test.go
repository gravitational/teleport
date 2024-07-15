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

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestNotifications(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	srv := env.server.Auth()
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	testRole, err := types.NewRole("auditors", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"auditor", "user"},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindNode, types.KindDatabase},
					Verbs:     services.RW(),
				},
			},
		},
	})
	require.NoError(t, err)

	username := "auditor"

	pack := proxy.authPack(t, username, []types.Role{testRole})

	// Describes a set of mock notifications to be created.
	// The number in each notification's description represents the creation order of the notifications.
	// eg. "auditor-3" will be the third notification created for auditor, and the third from last in the list (since it's the third oldest),
	testNotifications := []struct {
		userNotification   *notificationsv1.Notification
		globalNotification *notificationsv1.GlobalNotification
	}{
		{
			userNotification: newUserNotification(t, username, "auditor-1"),
		},
		{
			userNotification: newUserNotification(t, username, "auditor-2"),
		},
		{
			// Matcher matches by the role "auditors"
			globalNotification: newGlobalNotification(t, "auditor-3", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByRoles{
					ByRoles: &notificationsv1.ByRoles{
						Roles: []string{"auditors"},
					},
				},
			}),
		},
		{
			// This should not be returned in the list since it's not for this user.
			globalNotification: newGlobalNotification(t, "manager-1", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_ByRoles{
					ByRoles: &notificationsv1.ByRoles{
						Roles: []string{"managers"},
					},
				},
			}),
		},
		{
			userNotification: newUserNotification(t, username, "auditor-4"),
		},
		{
			// Matcher matches all.
			globalNotification: newGlobalNotification(t, "auditor-5", &notificationsv1.GlobalNotificationSpec{
				Matcher: &notificationsv1.GlobalNotificationSpec_All{
					All: true,
				},
			}),
		},
		{
			// This should not be returned in the list since it's not for this user.
			userNotification: newUserNotification(t, "manager", "manager-3"),
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
	}

	// Upsert last seen timestamp.
	lastSeenTimeString := "2024-05-08T19:00:47.836Z"
	_, err = pack.clt.PutJSON(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "lastseennotification"),
		UpsertUserLastSeenNotificationRequest{
			Time: lastSeenTimeString,
		})
	require.NoError(t, err)

	// Create the notifications.
	notificationIdMap := map[string]string{}

	for _, n := range testNotifications {
		// We add a small delay to ensure that the timestamps in the generated UUID's are all different and in the correct order.
		// This is to prevent flakiness caused by the notifications being created in such quick succession that the timestamps are the same.
		env.clock.Advance(50 * time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		// Create the notification in the backend.
		if n.globalNotification != nil {
			created, err := srv.CreateGlobalNotification(ctx, n.globalNotification)
			notificationIdMap[created.GetSpec().GetNotification().GetMetadata().GetLabels()[types.NotificationTitleLabel]] = created.GetMetadata().GetName()
			require.NoError(t, err)
			continue
		}
		created, err := srv.CreateUserNotification(ctx, n.userNotification)
		notificationIdMap[created.GetMetadata().GetLabels()[types.NotificationTitleLabel]] = created.GetMetadata().GetName()
		require.NoError(t, err)
	}

	expectedNotifications := []string{"auditor-7", "auditor-6", "auditor-5", "auditor-4", "auditor-3", "auditor-2", "auditor-1"}

	var fetchedNotifications []ui.Notification

	// Get a page of 4.
	notificationsResp, err := pack.clt.Get(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "notifications"), url.Values{
		"limit": []string{"4"},
	})
	require.NoError(t, err)
	unmarshaledNotificationsResp := unmarshalNotificationsResponse(t, notificationsResp.Bytes())
	fetchedNotifications = append(fetchedNotifications, unmarshaledNotificationsResp.Notifications...)

	expectedNextKeys := fmt.Sprintf("%s,%s",
		notificationIdMap["auditor-2"],
		notificationIdMap["auditor-3"])
	require.Equal(t, expectedNotifications[:4], notificationsToTitlesList(t, unmarshaledNotificationsResp.Notifications))
	require.Equal(t, expectedNextKeys, unmarshaledNotificationsResp.NextKey)

	// Get a page of 10, starting from the previously returned keys.
	notificationsResp, err = pack.clt.Get(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "notifications"), url.Values{
		"limit":    []string{"10"},
		"startKey": []string{unmarshaledNotificationsResp.NextKey},
	})
	require.NoError(t, err)
	unmarshaledNotificationsResp = unmarshalNotificationsResponse(t, notificationsResp.Bytes())
	fetchedNotifications = append(fetchedNotifications, unmarshaledNotificationsResp.Notifications...)

	require.Equal(t, expectedNotifications, notificationsToTitlesList(t, fetchedNotifications))
	require.Equal(t, "", unmarshaledNotificationsResp.NextKey)
	require.Equal(t, lastSeenTimeString, unmarshaledNotificationsResp.UserLastSeenNotification)

	// Mark the most recent notification as clicked.
	_, err = pack.clt.PutJSON(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "notificationstate"),
		upsertUserNotificationStateRequest{
			NotificationId:    notificationIdMap["auditor-7"],
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED,
		})
	require.NoError(t, err)

	// Mark the last notification as dismissed.
	_, err = pack.clt.PutJSON(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "notificationstate"),
		upsertUserNotificationStateRequest{
			NotificationId:    notificationIdMap["auditor-1"],
			NotificationState: notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED,
		})
	require.NoError(t, err)

	// List all notifications again.
	notificationsResp, err = pack.clt.Get(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "notifications"), url.Values{
		"limit": []string{"10"},
	})
	require.NoError(t, err)
	unmarshaledNotificationsResp = unmarshalNotificationsResponse(t, notificationsResp.Bytes())
	// Expect the first notification to be marked as clicked.
	require.True(t, unmarshaledNotificationsResp.Notifications[0].Clicked)
	// Expect the returned list to include all notifications except the last one.
	require.Equal(t, expectedNotifications[:6], notificationsToTitlesList(t, unmarshaledNotificationsResp.Notifications))

}

func unmarshalNotificationsResponse(t *testing.T, resp []byte) *GetNotificationsResponse {
	var notificationsResp *GetNotificationsResponse

	err := json.Unmarshal(resp, &notificationsResp)
	require.NoError(t, err)

	return notificationsResp
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
func notificationsToTitlesList(t *testing.T, notifications []ui.Notification) []string {
	t.Helper()
	var titles []string

	for _, notif := range notifications {
		title := notif.Title
		titles = append(titles, title)
	}

	return titles
}
