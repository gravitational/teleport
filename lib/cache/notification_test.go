// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/trace"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
)

// TestUserNotifications tests that CRUD operations on user notification resources are
// replicated from the backend to the cache.
func TestUserNotifications(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*notificationsv1.Notification]{
		newResource: func(name string) (*notificationsv1.Notification, error) {
			return newUserNotification(t, name), nil
		},
		create: func(ctx context.Context, item *notificationsv1.Notification) error {
			_, err := p.notifications.CreateUserNotification(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.notifications.ListUserNotifications,
		cacheList: p.cache.ListUserNotifications,
		deleteAll: p.notifications.DeleteAllUserNotifications,
	})
}

// TestGlobalNotifications tests that CRUD operations on global notification resources are
// replicated from the backend to the cache.
func TestGlobalNotifications(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*notificationsv1.GlobalNotification]{
		newResource: func(name string) (*notificationsv1.GlobalNotification, error) {
			return newGlobalNotification(t, name), nil
		},
		create: func(ctx context.Context, item *notificationsv1.GlobalNotification) error {
			_, err := p.notifications.CreateGlobalNotification(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.notifications.ListGlobalNotifications,
		cacheList: p.cache.ListGlobalNotifications,
		deleteAll: p.notifications.DeleteAllGlobalNotifications,
	})
}
