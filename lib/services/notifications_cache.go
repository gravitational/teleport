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
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

const (
	// notificationKey is the key for a user-specific notification in the format of <username>/<notification uuid>.
	// This index is only used by the user notifications cache. Since UUIDv7's contain a timestamp and are lexicographically sortable
	// by date, this is what will be used to sort by date.
	notificationKey = "Key"
	// notificationID is the uuid of a notification.
	notificationID = "ID"
)

// NotificationGetter defines the interface for fetching notifications.
type NotificationGetter interface {
	// ListUserNotifications returns a paginated list of user-specific notifications for all users.
	ListUserNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.Notification, string, error)
	// ListGlobalNotifications returns a paginated list of global notifications.
	ListGlobalNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.GlobalNotification, string, error)
}

// UserNotificationsCacheConfig holds the configuration parameters for both [UserNotificationCache] and [GlobalNotificationCache].
type NotificationCacheConfig struct {
	// Clock is a clock for time-related operation.
	Clock clockwork.Clock
	// Events is an event system client.
	Events types.Events
	// Getter is an notification getter client.
	Getter NotificationGetter
}

// CheckAndSetDefaults validates the config and provides reasonable defaults for optional fields.
func (c *NotificationCacheConfig) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Events == nil {
		return trace.BadParameter("notification cache config missing event system client")
	}

	if c.Getter == nil {
		return trace.BadParameter("notification cache config missing notifications getter")
	}

	return nil
}

// UserNotificationCache is a custom cache for user-specific notifications, this is to allow
// fetching notifications by date in descending order.
type UserNotificationCache struct {
	rw           sync.RWMutex
	cfg          NotificationCacheConfig
	primaryCache *sortcache.SortCache[*notificationsv1.Notification]
	ttlCache     *utils.FnCache
	initC        chan struct{}
	closeContext context.Context
	cancel       context.CancelFunc
}

// GlobalNotificationCache is a custom cache for user-specific notifications, this is to allow
// fetching notifications by date in descending order.
type GlobalNotificationCache struct {
	rw           sync.RWMutex
	cfg          NotificationCacheConfig
	primaryCache *sortcache.SortCache[*notificationsv1.GlobalNotification]
	ttlCache     *utils.FnCache
	initC        chan struct{}
	closeContext context.Context
	cancel       context.CancelFunc
}

// NewUserNotificationCache sets up a new [UserNotificationCache] instance based on the supplied
// configuration. The cache is initialized asychronously in the background, so while it is
// safe to read from it immediately, performance is better after the cache properly initializes.
func NewUserNotificationCache(cfg NotificationCacheConfig) (*UserNotificationCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context: ctx,
		TTL:     15 * time.Second,
		Clock:   cfg.Clock,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	c := &UserNotificationCache{
		cfg:          cfg,
		ttlCache:     ttlCache,
		initC:        make(chan struct{}),
		closeContext: ctx,
		cancel:       cancel,
	}

	if _, err := newResourceWatcher(ctx, c, ResourceWatcherConfig{
		Component: "user-notification-cache",
		Client:    cfg.Events,
	}); err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	return c, nil
}

// NewGlobalNotificationCache sets up a new [GlobalNotificationCache] instance based on the supplied
// configuration. The cache is initialized asychronously in the background, so while it is
// safe to read from it immediately, performance is better after the cache properly initializes.
func NewGlobalNotificationCache(cfg NotificationCacheConfig) (*GlobalNotificationCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context: ctx,
		TTL:     15 * time.Second,
		Clock:   cfg.Clock,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	c := &GlobalNotificationCache{
		cfg:          cfg,
		ttlCache:     ttlCache,
		initC:        make(chan struct{}),
		closeContext: ctx,
		cancel:       cancel,
	}

	if _, err := newResourceWatcher(ctx, c, ResourceWatcherConfig{
		Component: "global-notification-cache",
		Client:    cfg.Events,
	}); err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	return c, nil
}

// StreamUserNotifications returns a stream with the user-specific notifications in the cache for a specified user, sorted from newest to oldest.
// We use streams here as it's a convenient way for us to construct pages to be returned to the UI one item at a time in combination with global notifications.
func (c *UserNotificationCache) StreamUserNotifications(ctx context.Context, username string, startKey string) stream.Stream[*notificationsv1.Notification] {
	if username == "" {
		return stream.Fail[*notificationsv1.Notification](trace.BadParameter("username is required for fetching user notifications"))
	}

	cache, err := c.read(ctx)
	if err != nil {
		return stream.Fail[*notificationsv1.Notification](trace.Wrap(err))
	}

	if !cache.HasIndex(notificationKey) {
		return stream.Fail[*notificationsv1.Notification](trace.Errorf("user notifications cache was not configured with index %q (this is a bug)", notificationKey))
	}

	endKey := username + string(backend.Separator)
	// Get the initial startKey if it wasn't provided.
	if startKey == "" {
		startKey = sortcache.NextKey(endKey)
	} else {
		// The sortcache expects the key to be in <username>/<uuid> format, so we prepend the username since the startKey passed into this function will just be a UUID.
		startKey = fmt.Sprintf("%s/%s", username, startKey)
	}

	var done bool
	return stream.PageFunc(func() ([]*notificationsv1.Notification, error) {
		if done {
			return nil, io.EOF
		}
		notifications, nextKey := c.primaryCache.DescendPaginated(notificationKey, startKey, endKey, 50)
		startKey = nextKey
		done = nextKey == ""

		// Return copies of the notification to prevent mutating the original.
		clonedNotifications := make([]*notificationsv1.Notification, 0, len(notifications))
		for _, notification := range notifications {
			clonedNotifications = append(clonedNotifications, apiutils.CloneProtoMsg(notification))
		}
		return clonedNotifications, nil
	})
}

// fetch initializes a sortcache with all existing user-specific notifications. This is used to set up the initialize the primary cache, and
// to create a temporary cache as a fallback in case the primary cache is ever unhealthy.
func (c *UserNotificationCache) fetch(ctx context.Context) (*sortcache.SortCache[*notificationsv1.Notification], error) {
	cache := sortcache.New(sortcache.Config[*notificationsv1.Notification]{
		Indexes: map[string]func(*notificationsv1.Notification) string{
			notificationKey: func(n *notificationsv1.Notification) string {
				return GetUserSpecificKey(n)
			},
			notificationID: func(n *notificationsv1.Notification) string {
				return n.GetMetadata().GetName()
			},
		},
	})

	var startKey string
	for {
		notifications, nextKey, err := c.cfg.Getter.ListUserNotifications(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, n := range notifications {
			if evicted := cache.Put(n); evicted != 0 {
				// this warning, if it appears, means that we configured our indexes incorrectly and one notification is overwriting another.
				// the most likely explanation is that one of our indexes is missing the notification id suffix we typically use.
				slog.WarnContext(ctx, "Notification conflicted with other notifications during cache fetch. This is a bug and may result in notifications not appearing the in UI correctly.", "notification", n.GetMetadata().GetName(), "num_clashing_notifications", evicted)
			}
		}

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}

	return cache, nil
}

// GetUserSpecificKey returns the key for a user-specific notification in <username>/<notification uuid> format.
func GetUserSpecificKey(n *notificationsv1.Notification) string {
	username := n.GetSpec().GetUsername()
	id := n.GetMetadata().GetName()

	return fmt.Sprintf("%s/%s", username, id)
}

// read gets a read-only view into a valid cache state. it prefers reading from the primary cache, but will fallback
// to a periodically reloaded temporary state when the primary state is unhealthy.
func (c *UserNotificationCache) read(ctx context.Context) (*sortcache.SortCache[*notificationsv1.Notification], error) {
	c.rw.RLock()
	primary := c.primaryCache
	c.rw.RUnlock()

	// primary cache state is healthy, so use that. note that we don't protect access to the sortcache itself
	// via our rw lock. sortcaches have their own internal locking.  we just use our lock to protect the *pointer*
	// to the sortcache.
	if primary != nil {
		return primary, nil
	}

	temp, err := utils.FnCacheGet(ctx, c.ttlCache, "user-notification-cache", func(ctx context.Context) (*sortcache.SortCache[*notificationsv1.Notification], error) {
		return c.fetch(ctx)
	})

	// primary may have been concurrently loaded. if it was, prefer using that.
	c.rw.RLock()
	primary = c.primaryCache
	c.rw.RUnlock()

	if primary != nil {
		return primary, nil
	}

	return temp, trace.Wrap(err)
}

// StreamGlobalNotifications returns a stream with all the global notifications in the cache, sorted from newest to oldest.
func (c *GlobalNotificationCache) StreamGlobalNotifications(ctx context.Context, startKey string) stream.Stream[*notificationsv1.GlobalNotification] {
	cache, err := c.read(ctx)
	if err != nil {
		return stream.Fail[*notificationsv1.GlobalNotification](trace.Wrap(err))
	}

	if !cache.HasIndex(notificationID) {
		return stream.Fail[*notificationsv1.GlobalNotification](trace.Errorf("global notifications cache was not configured with index %q (this is a bug)", notificationID))
	}

	var done bool
	return stream.PageFunc(func() ([]*notificationsv1.GlobalNotification, error) {
		if done {
			return nil, io.EOF
		}
		notifications, nextKey := c.primaryCache.DescendPaginated(notificationID, startKey, "", 50)
		startKey = nextKey
		done = nextKey == ""

		// Return copies of the notification to prevent mutating the original.
		clonedNotifications := make([]*notificationsv1.GlobalNotification, 0, len(notifications))
		for _, notification := range notifications {
			clonedNotifications = append(clonedNotifications, apiutils.CloneProtoMsg(notification))
		}
		return clonedNotifications, nil
	})
}

// fetch initializes a sortcache with all existing global notifications. This is used to set up the initialize the primary cache, and
// to create a temporary cache as a fallback in case the primary cache is ever unhealthy.
func (c *GlobalNotificationCache) fetch(ctx context.Context) (*sortcache.SortCache[*notificationsv1.GlobalNotification], error) {
	cache := sortcache.New(sortcache.Config[*notificationsv1.GlobalNotification]{
		Indexes: map[string]func(*notificationsv1.GlobalNotification) string{
			notificationID: func(gn *notificationsv1.GlobalNotification) string {
				return gn.GetMetadata().GetName()
			},
		},
	})

	var startKey string
	for {
		notifications, nextKey, err := c.cfg.Getter.ListGlobalNotifications(ctx, 0, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, n := range notifications {
			if evicted := cache.Put(n); evicted != 0 {
				// this warning, if it appears, means that we configured our indexes incorrectly and one notification is overwriting another.
				// the most likely explanation is that one of our indexes is missing the notification id suffix we typically use.
				slog.WarnContext(ctx, "Notification conflicted with other notifications during cache fetch. This is a bug and may result in notifications not appearing the in UI correctly.", "notification", n.GetMetadata().GetName(), "num_clashing_notifications", evicted)
			}
		}

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}

	return cache, nil
}

// read gets a read-only view into a valid cache state. it prefers reading from the primary cache, but will fallback
// to a periodically reloaded temporary state when the primary state is unhealthy.
func (c *GlobalNotificationCache) read(ctx context.Context) (*sortcache.SortCache[*notificationsv1.GlobalNotification], error) {
	c.rw.RLock()
	primary := c.primaryCache
	c.rw.RUnlock()

	// primary cache state is healthy, so use that. note that we don't protect access to the sortcache itself
	// via our rw lock. sortcaches have their own internal locking.  we just use our lock to protect the *pointer*
	// to the sortcache.
	if primary != nil {
		return primary, nil
	}

	temp, err := utils.FnCacheGet(ctx, c.ttlCache, "global-notification-cache", func(ctx context.Context) (*sortcache.SortCache[*notificationsv1.GlobalNotification], error) {
		return c.fetch(ctx)
	})

	// primary may have been concurrently loaded. if it was, prefer using that.
	c.rw.RLock()
	primary = c.primaryCache
	c.rw.RUnlock()

	if primary != nil {
		return primary, nil
	}

	return temp, trace.Wrap(err)
}

// --- the below methods implement the resourceCollector interface ---

// resourceKinds is part of the resourceCollector interface and is used to configure the event watcher
// that monitors for notification modifications.
func (c *UserNotificationCache) resourceKinds() []types.WatchKind {
	return []types.WatchKind{
		{
			Kind: types.KindNotification,
		},
	}
}
func (c *GlobalNotificationCache) resourceKinds() []types.WatchKind {
	return []types.WatchKind{
		{
			Kind: types.KindGlobalNotification,
		},
	}
}

// getResourcesAndUpdateCurrent is part of the resourceCollector interface and is called when the
// event stream for the cache has been initialized to trigger setup of the initial primary cache state.
func (c *UserNotificationCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	cache, err := c.fetch(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	c.rw.Lock()
	defer c.rw.Unlock()
	c.primaryCache = cache
	return nil
}
func (c *GlobalNotificationCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	cache, err := c.fetch(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	c.rw.Lock()
	defer c.rw.Unlock()
	c.primaryCache = cache
	return nil
}

// processEventsAndUpdateCurrent is part of the resourceCollector interface and is used to update the
// primary cache state when modification events occur.
func (c *UserNotificationCache) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	c.rw.RLock()
	cache := c.primaryCache
	c.rw.RUnlock()

	for _, event := range events {
		switch event.Type {
		case types.OpPut:
			// Since the EventsService watcher currently only supports legacy resources, we had to use types.Resource153ToLegacy() when parsing the event
			// to transform the notification into a legacy resource. We now have to use Unwrap() to get the original RFD153-style notification out and add it to the cache.
			resource153, ok := event.Resource.(types.Resource153Unwrapper)
			if !ok {
				slog.WarnContext(ctx, "Unexpected resource type in event (expected types.Resource153Unwrapper)", "resource_type", logutils.TypeAttr(resource153))
				continue
			}
			resource := resource153.Unwrap()

			notification, ok := resource.(*notificationsv1.Notification)
			if !ok {
				slog.WarnContext(ctx, "Unexpected resource type in event (expected *notificationsv1.Notification)", "resource_type", logutils.TypeAttr(resource))
				continue
			}
			if evicted := cache.Put(notification); evicted > 1 {
				slog.WarnContext(ctx, "Processing of put event for notification resulted in multiple cache evictions (this is a bug).", "notification", notification.GetMetadata().GetName())
			}
		case types.OpDelete:
			cache.Delete(notificationID, event.Resource.GetName())
		default:
			slog.WarnContext(ctx, "Unexpected event variant", "event", event.Type)
		}
	}
}

func (c *GlobalNotificationCache) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	c.rw.RLock()
	cache := c.primaryCache
	c.rw.RUnlock()

	for _, event := range events {
		switch event.Type {
		case types.OpPut:
			resource153, ok := event.Resource.(types.Resource153Unwrapper)
			if !ok {
				slog.WarnContext(ctx, "Unexpected resource type in event (expected types.Resource153Unwrapper)", "resource_type", logutils.TypeAttr(resource153))
				continue
			}
			resource := resource153.Unwrap()

			globalNotification, ok := resource.(*notificationsv1.GlobalNotification)
			if !ok {
				slog.WarnContext(ctx, "Unexpected resource type in event (expected *notificationsv1.GlobalNotification)", "resource_type", logutils.TypeAttr(resource))
				continue
			}
			if evicted := cache.Put(globalNotification); evicted > 1 {
				slog.WarnContext(ctx, "Processing of put event for notification resulted in multiple cache evictions (this is a bug).", "notification", globalNotification.GetMetadata().GetName())
			}
		case types.OpDelete:
			cache.Delete(notificationID, event.Resource.GetName())
		default:
			slog.WarnContext(ctx, "Unexpected event variant", "event", event.Type)
		}
	}
}

// notifyStale is part of the resourceCollector interface and is used to inform
// the notification cache that its view is outdated (presumably due to issues with
// the event stream).
func (c *UserNotificationCache) notifyStale() {
	c.rw.Lock()
	defer c.rw.Unlock()
	if c.primaryCache == nil {
		return
	}
	c.primaryCache = nil
	c.initC = make(chan struct{})
}
func (c *GlobalNotificationCache) notifyStale() {
	c.rw.Lock()
	defer c.rw.Unlock()
	if c.primaryCache == nil {
		return
	}
	c.primaryCache = nil
	c.initC = make(chan struct{})
}

// initializationChan is part of the resourceCollector interface and gets the channel
// used to signal that the notification cache has been initialized.
func (c *UserNotificationCache) initializationChan() <-chan struct{} {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.initC
}
func (c *GlobalNotificationCache) initializationChan() <-chan struct{} {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.initC
}

// Close terminates the background process that keeps the notification cache up to
// date, and terminates any inflight load operations.
func (c *UserNotificationCache) Close() error {
	c.cancel()
	return nil
}
func (c *GlobalNotificationCache) Close() error {
	c.cancel()
	return nil
}
