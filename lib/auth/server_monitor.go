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
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// timeCheckCycle is the period when local times are compared
	// for collected resources from heartbeats, and notification must be added.
	timeCheckCycle = 10 * time.Minute
	// timeShiftThreshold is the duration threshold for triggering a warning
	// if the time difference exceeds this threshold.
	timeShiftThreshold = time.Minute
)

// resourceWithLocalTime is minimal interface for resource with local time.
type resourceWithLocalTime interface {
	types.Resource

	// GetLocalTime gets the local time of the current server.
	GetLocalTime() time.Time
}

// resourceMonitor stores info about resource and time difference with local time.
type resourceMonitor struct {
	resource resourceWithLocalTime
	diff     time.Duration
}

// MonitorNodeInfos consumes heartbeat events of other services to periodically
// compare the auth server time with the time of remote services,
// and notifying about the time difference between servers.
func (a *Server) MonitorNodeInfos(ctx context.Context) error {
	resources := make(map[string]resourceMonitor)
	ticker := a.clock.NewTicker(timeCheckCycle)

	watcher, err := a.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindNode},
			{Kind: types.KindWindowsDesktopService},
			{Kind: types.KindKubeServer},
			{Kind: types.KindAppServer},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.Chan():
			err := upsertGlobalNotification(ctx, a.Services, generateNotificationMessage(resources))
			if err != nil {
				slog.ErrorContext(ctx, "can't set notification about time difference", err)
			}
		case event := <-watcher.Events():
			res, ok := event.Resource.(resourceWithLocalTime)
			if !ok {
				continue
			}
			// Previous version of the servers don't have this parameter, we have to skip them.
			if res.GetLocalTime().IsZero() {
				continue
			}

			if event.Type == types.OpPut {
				now := a.clock.Now().UnixNano()
				localTime := res.GetLocalTime().UnixNano()
				duration := time.Duration(localTime - now)

				if (duration > 0 && duration > timeShiftThreshold) || (duration < 0 && -duration > timeShiftThreshold) {
					resources[res.GetName()] = resourceMonitor{resource: res, diff: duration}
					slog.WarnContext(ctx, "server time difference detected",
						"server", res.GetName(), "difference", durationText(duration))
				}
			} else if event.Type == types.OpDelete {
				delete(resources, res.GetName())
			}
		}
	}
}

// upsertGlobalNotification sets predefined global notification for notifying the issues with the cluster
// servers related to the time difference in nodes.
func upsertGlobalNotification(ctx context.Context, services *Services, text string) error {
	_, err := services.UpsertGlobalNotification(ctx, &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_All{
				All: true,
			},
			Notification: &notificationsv1.Notification{
				Spec: &notificationsv1.NotificationSpec{},
				Metadata: &headerv1.Metadata{
					Name: "cluster-monitor-time-sync",
					Labels: map[string]string{
						types.NotificationTitleLabel: text,
					},
				},
			},
		},
	})
	return trace.Wrap(err)
}

// generateNotificationMessage formats the notification message for the user with detailed information
// about the server name and time difference in comparison with the auth node.
func generateNotificationMessage(resources map[string]resourceMonitor) string {
	serverAlias := map[string]string{
		types.KindNode:                  "Node",
		types.KindWindowsDesktopService: "WindowsDesktopService",
		types.KindKubeServer:            "KubeServer",
		types.KindAppServer:             "AppServer",
	}
	var serverMessages []string
	for _, server := range resources {
		alias, ok := serverAlias[server.resource.GetKind()]
		if ok {
			serverMessages = append(serverMessages, fmt.Sprintf(
				"%s(%s) is %s",
				alias,
				server.resource.GetName(),
				durationText(server.diff)),
			)
		}
	}

	return "Incorrect system clock detected in the cluster, which may lead to certificate validation issues.\n" +
		"Ensure that the clock is accurate on all nodes to avoid potential access problems.\n" +
		"List of servers with a time shift: \n" + strings.Join(serverMessages, "\n")
}

// durationText formats specified duration to text by adding suffix ahead/behind and
// transforms nanoseconds to formatted time with hours, minutes, seconds.
func durationText(duration time.Duration) string {
	if duration > 0 {
		return fmt.Sprintf("%s ahead", duration.String())
	} else {
		return fmt.Sprintf("%s behind", (-duration).String())
	}
}
