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

package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type notificationServices struct {
	types.Events
	services.Notifications
}

func newUserNotificationPack(t *testing.T) (notificationServices, *services.UserNotificationCache) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	notificationService, err := local.NewNotificationsService(bk, clock)
	require.NoError(t, err)

	svcs := notificationServices{
		Events:        local.NewEventsService(bk),
		Notifications: notificationService,
	}

	cache, err := services.NewUserNotificationCache(services.NotificationCacheConfig{
		Events: svcs,
		Getter: svcs,
	})
	require.NoError(t, err)

	return svcs, cache
}

func newGlobalNotificationPack(t *testing.T) (notificationServices, *services.GlobalNotificationCache) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	notificationService, err := local.NewNotificationsService(bk, clock)
	require.NoError(t, err)

	svcs := notificationServices{
		Events:        local.NewEventsService(bk),
		Notifications: notificationService,
	}

	cache, err := services.NewGlobalNotificationCache(services.NotificationCacheConfig{
		Events: svcs,
		Getter: svcs,
	})
	require.NoError(t, err)

	return svcs, cache
}

// TestUserNotificationsCache verifies the expected behaviors of the user-specific notifications cache.
func TestUserNotificationsCache(t *testing.T) {
	t.Parallel()

	svcs, cache := newUserNotificationPack(t)
	defer cache.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// To test that the streaming of notifications for a user is correct, we will have the mock list of user-specific notifications
	// contain notifications for two users: alice and bob.
	usernameA := "alice"
	usernameB := "bob"

	// Describe a set of mock user notifications to be created.
	// The number in the description represents the order in which they are created, eg. alice-1 is the first (and thus oldest) notification to be created for alice.
	testNotifications := []struct {
		username    string
		description string
	}{
		{
			username:    usernameB,
			description: "bob-1",
		},
		{
			username:    usernameB,
			description: "bob-2",
		},
		{
			username:    usernameA,
			description: "alice-1",
		},
		{
			username:    usernameB,
			description: "bob-3",
		},
		{
			username:    usernameB,
			description: "bob-4",
		},
		{
			username:    usernameA,
			description: "alice-2",
		},
		{
			username:    usernameA,
			description: "alice-3",
		},
		{
			username:    usernameA,
			description: "alice-4",
		},
		{
			username:    usernameB,
			description: "bob-5",
		},
		{
			username:    usernameA,
			description: "alice-5",
		},
	}

	// We want to keep the notification ID's of the created notifications for later to test deleting notifications.
	var notificationIdsA []string
	var notificationIdsB []string

	// Create the user-specific notifications, with a 50ms delay between each one.
	for _, n := range testNotifications {
		// We add a delay to ensure that the timestamps in the generated UUID's are all different and in the correct order.
		time.Sleep(50 * time.Millisecond)
		notification := newUserNotification(t, n.description)
		// Create the notification in the backend.
		created, err := svcs.CreateUserNotification(ctx, n.username, notification)
		require.NoError(t, err)
		if n.username == usernameA {
			notificationIdsA = append(notificationIdsA, created.GetMetadata().GetName())
		} else {
			notificationIdsB = append(notificationIdsB, created.GetMetadata().GetName())
		}
	}

	// Wait and verify that the cache is populated with all the items.
	timeout := time.After(time.Second * 30)

	for {
		streamA := cache.StreamUserNotifications(ctx, usernameA, "")
		streamB := cache.StreamUserNotifications(ctx, usernameB, "")
		collectedStreamA, err := stream.Collect(streamA)
		require.NoError(t, err)
		streamA.Done()
		collectedStreamB, err := stream.Collect(streamB)
		require.NoError(t, err)
		streamB.Done()

		if (len(collectedStreamA) + len(collectedStreamB)) == len(testNotifications) {
			break
		}

		select {
		case <-timeout:
			require.FailNow(t, "timeout waiting for user notifications cache to populate")
		case <-time.After(time.Millisecond * 200):
		}
	}

	// This is the startKey of the third item for usernameA, we will use this to test that fetching starting from a specified startKey works properly.
	var usernameAThirdItemStartKey string
	streamA := cache.StreamUserNotifications(ctx, usernameA, "")
	streamA.Next()
	streamA.Next()
	streamA.Next()
	usernameAThirdItemStartKey = services.GetUserSpecificKey(streamA.Item())

	var usernameBThirdItemStartKey string
	streamB := cache.StreamUserNotifications(ctx, usernameB, "")
	streamB.Next()
	streamB.Next()
	streamB.Next()
	usernameBThirdItemStartKey = services.GetUserSpecificKey(streamB.Item())

	testCases := []struct {
		testName                         string
		username                         string
		startKey                         string
		expectedNotificationDescriptions []string
	}{
		{
			testName: "correctly fetches sorted notifications for usernameA with no startKey",
			username: usernameA,
			// Since alice-1 is the oldest, it should be at the end of the list.
			expectedNotificationDescriptions: []string{"alice-5", "alice-4", "alice-3", "alice-2", "alice-1"},
		},
		{
			testName:                         "correctly fetches sorted notifications for usernameB with no startKey",
			username:                         usernameB,
			expectedNotificationDescriptions: []string{"bob-5", "bob-4", "bob-3", "bob-2", "bob-1"},
		}, {
			testName:                         "correctly fetches sorted notifications for usernameA with a startKey",
			username:                         usernameA,
			startKey:                         usernameAThirdItemStartKey,
			expectedNotificationDescriptions: []string{"alice-3", "alice-2", "alice-1"},
		}, {
			testName:                         "correctly fetches sorted notifications for usernameB with a startKey",
			username:                         usernameB,
			startKey:                         usernameBThirdItemStartKey,
			expectedNotificationDescriptions: []string{"bob-3", "bob-2", "bob-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			var out []string
			notifsStream := cache.StreamUserNotifications(ctx, tc.username, tc.startKey)
			for notifsStream.Next() {
				desc := notifsStream.Item().GetMetadata().GetLabels()["description"]
				out = append(out, desc)
			}
			notifsStream.Done()
			require.Equal(t, tc.expectedNotificationDescriptions, out)
		})
	}

	// Verify that an error is returned if a username isn't provided.
	streamA = cache.StreamUserNotifications(ctx, "", "")
	_, err := stream.Collect(streamA)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	streamA.Done()

	// Verify that delete events are correctly processed.
	// For usernameA, we will test deleting only the two most recent notifications.
	svcs.DeleteUserNotification(ctx, usernameA, notificationIdsA[len(notificationIdsA)-1])
	svcs.DeleteUserNotification(ctx, usernameA, notificationIdsA[len(notificationIdsA)-2])

	// For usernameB, we will test deleting all notifications.
	for _, id := range notificationIdsB {
		svcs.DeleteUserNotification(ctx, usernameB, id)
	}

	// Wait and verify that the cache has been updated.
	timeout = time.After(time.Second * 30)

	for {
		streamA := cache.StreamUserNotifications(ctx, usernameA, "")
		streamB := cache.StreamUserNotifications(ctx, usernameB, "")
		collectedStreamA, err := stream.Collect(streamA)
		require.NoError(t, err)
		streamA.Done()
		collectedStreamB, err := stream.Collect(streamB)
		require.NoError(t, err)
		streamB.Done()

		if len(collectedStreamA) == (len(notificationIdsA)-2) &&
			len(collectedStreamB) == 0 {
			break
		}

		select {
		case <-timeout:
			require.FailNow(t, "timeout waiting for user notifications cache to update")
		case <-time.After(time.Millisecond * 200):
		}
	}

	// Verify that the correct items were deleted.
	var out []string
	expected := []string{"alice-3", "alice-2", "alice-1"}
	notifsStream := cache.StreamUserNotifications(ctx, usernameA, "")
	for notifsStream.Next() {
		desc := notifsStream.Item().GetMetadata().GetLabels()["description"]
		out = append(out, desc)
	}
	notifsStream.Done()
	require.Equal(t, expected, out)
}

// TestGlobalNotificationsCache verifies the expected behaviors of the global notifications cache.
func TestGlobalNotificationsCache(t *testing.T) {
	t.Parallel()

	svcs, cache := newGlobalNotificationPack(t)
	defer cache.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testNotificationDescriptions := []string{
		"gn-1",
		"gn-2",
		"gn-3",
		"gn-4",
		"gn-5",
	}

	// We want to keep the notification ID's of the created notifications for later to test deleting notifications.
	var notificationIds []string

	// Create the notifications, with a 50ms delay between each one.
	for _, gn := range testNotificationDescriptions {
		time.Sleep(50 * time.Millisecond)
		notification := newGlobalNotification(t, gn)
		// Create the notification in the backend.
		created, err := svcs.CreateGlobalNotification(ctx, notification)
		notificationIds = append(notificationIds, created.GetMetadata().GetName())
		require.NoError(t, err)
	}

	// Wait and verify that the cache is populated with all the items.
	timeout := time.After(time.Second * 30)

	for {
		gnStream := cache.StreamGlobalNotifications(ctx, "")
		collectedStream, err := stream.Collect(gnStream)
		require.NoError(t, err)
		gnStream.Done()

		if len(collectedStream) == len(testNotificationDescriptions) {
			break
		}

		select {
		case <-timeout:
			require.FailNow(t, "timeout waiting for global notifications cache to populate")
		case <-time.After(time.Millisecond * 200):
		}
	}

	// This is the startKey of the third item in the list.
	var thirdItemStartKey string
	gnStream := cache.StreamGlobalNotifications(ctx, "")
	gnStream.Next()
	gnStream.Next()
	gnStream.Next()
	thirdItemStartKey = gnStream.Item().GetMetadata().GetName()
	gnStream.Done()

	// Test that streaming global notifications with no startKey works correctly
	var out []string
	expected := []string{"gn-5", "gn-4", "gn-3", "gn-2", "gn-1"}
	gnStream = cache.StreamGlobalNotifications(ctx, "")
	for gnStream.Next() {
		desc := gnStream.Item().GetSpec().GetNotification().GetMetadata().GetLabels()["description"]
		out = append(out, desc)
	}
	require.Equal(t, expected, out)
	gnStream.Done()

	// Test that streaming global notifications with a startKeys works correctly
	out = []string{}
	expected = []string{"gn-3", "gn-2", "gn-1"}
	gnStream = cache.StreamGlobalNotifications(ctx, thirdItemStartKey)
	for gnStream.Next() {
		desc := gnStream.Item().GetSpec().GetNotification().GetMetadata().GetLabels()["description"]
		out = append(out, desc)
	}
	require.Equal(t, expected, out)
	gnStream.Done()

	// Verify that delete events are correctly processed.

	// Delete all the notifications.
	for _, id := range notificationIds {
		svcs.DeleteGlobalNotification(ctx, id)
	}

	// Wait and verify that the cache has been updated.
	timeout = time.After(time.Second * 30)

	for {
		gnStream := cache.StreamGlobalNotifications(ctx, "")
		collectedStream, err := stream.Collect(gnStream)
		require.NoError(t, err)
		gnStream.Done()

		if len(collectedStream) == 0 {
			break
		}

		select {
		case <-timeout:
			require.FailNow(t, "timeout waiting for global notifications cache to update")
		case <-time.After(time.Millisecond * 200):
		}
	}
}

func newUserNotification(t *testing.T, description string) *notificationsv1.Notification {
	t.Helper()

	notification := notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec:    &notificationsv1.NotificationSpec{},
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
