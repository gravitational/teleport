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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type userNotificationIndex string

const userNotificationNameIndex userNotificationIndex = "name"

func newUserNotificationCollection(upstream services.NotificationGetter, w types.WatchKind) (*collection[*notificationsv1.Notification, userNotificationIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter NotificationGetter")
	}

	return &collection[*notificationsv1.Notification, userNotificationIndex]{
		store: newStore(
			proto.CloneOf[*notificationsv1.Notification],
			map[userNotificationIndex]func(*notificationsv1.Notification) string{
				userNotificationNameIndex: func(r *notificationsv1.Notification) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*notificationsv1.Notification, error) {
			var notifications []*notificationsv1.Notification
			var startKey string
			for {
				notifs, nextKey, err := upstream.ListUserNotifications(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				notifications = append(notifications, notifs...)

				if nextKey == "" {
					break
				}
				startKey = nextKey
			}

			return notifications, nil
		},
		watch: w,
	}, nil
}

// ListUserNotifications returns a paginated list of user-specific notifications for all users.
func (c *Cache) ListUserNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.Notification, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListUserNotifications")
	defer span.End()

	lister := genericLister[*notificationsv1.Notification, userNotificationIndex]{
		cache:        c,
		collection:   c.collections.userNotifications,
		index:        userNotificationNameIndex,
		upstreamList: c.Config.Notifications.ListUserNotifications,
		nextToken: func(t *notificationsv1.Notification) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, startKey)
	return out, next, trace.Wrap(err)
}

type globalNotificationIndex string

const globalNotificationNameIndex globalNotificationIndex = "name"

func newGlobalNotificationCollection(upstream services.NotificationGetter, w types.WatchKind) (*collection[*notificationsv1.GlobalNotification, globalNotificationIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter NotificationGetter")
	}

	return &collection[*notificationsv1.GlobalNotification, globalNotificationIndex]{
		store: newStore(
			proto.CloneOf[*notificationsv1.GlobalNotification],
			map[globalNotificationIndex]func(*notificationsv1.GlobalNotification) string{
				globalNotificationNameIndex: func(r *notificationsv1.GlobalNotification) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*notificationsv1.GlobalNotification, error) {
			var notifications []*notificationsv1.GlobalNotification
			var startKey string
			for {
				notifs, nextKey, err := upstream.ListGlobalNotifications(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				notifications = append(notifications, notifs...)

				if nextKey == "" {
					break
				}
				startKey = nextKey
			}

			return notifications, nil
		},
		watch: w,
	}, nil
}

// ListGlobalNotifications returns a paginated list of global notifications.
func (c *Cache) ListGlobalNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.GlobalNotification, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListGlobalNotifications")
	defer span.End()

	lister := genericLister[*notificationsv1.GlobalNotification, globalNotificationIndex]{
		cache:        c,
		collection:   c.collections.globalNotifications,
		index:        globalNotificationNameIndex,
		upstreamList: c.Config.Notifications.ListGlobalNotifications,
		nextToken: func(t *notificationsv1.GlobalNotification) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, startKey)
	return out, next, trace.Wrap(err)
}
