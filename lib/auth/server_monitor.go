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
	insecurerand "math/rand"
	"strings"
	"time"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
)

const (
	// timeCheckCycle is the period when local times are compared
	// for collected resources from heartbeats, and notification must be added.
	timeCheckCycle = 10 * time.Minute
	// timeShiftThreshold is the duration threshold for triggering a warning
	// if the time difference exceeds this threshold.
	timeShiftThreshold = time.Minute
)

// inventoryMonitor stores info about resource and time difference with local time.
type inventoryMonitor struct {
	serverID string
	services types.SystemRoles
	diff     time.Duration
}

// String returns the inventory representation.
func (res *inventoryMonitor) String() string {
	return fmt.Sprintf(
		"%s[%s] is %s",
		res.serverID,
		res.services.String(),
		durationText(res.diff),
	)
}

// MonitorNodeInfos consumes heartbeat events of other services to periodically
// compare the auth server time with the time of remote services,
// and notifying about the time difference between servers.
func (a *Server) MonitorNodeInfos(ctx context.Context) error {
	ticker := a.clock.NewTicker(timeCheckCycle)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.Chan():
			var inventories []inventoryMonitor
			a.inventory.Iter(func(handle inventory.UpstreamHandle) {
				info := handle.Hello()
				diff, err := handle.TimeReconciliation(ctx, insecurerand.Uint64())
				if err != nil {
					slog.ErrorContext(ctx, "error getting time reconciliation")
				}

				if (diff > 0 && diff > timeShiftThreshold) || (diff < 0 && -diff > timeShiftThreshold) {
					inventories = append(inventories, inventoryMonitor{
						serverID: info.GetServerID(),
						services: info.GetServices(),
						diff:     diff,
					})
					slog.WarnContext(ctx, "server time difference detected",
						"server", info.GetServerID(),
						"services", info.GetServices(),
						"difference", durationText(diff),
					)
				}
			})

			if len(inventories) > 0 {
				err := upsertGlobalNotification(ctx, a.Services, generateNotificationMessage(inventories))
				if err != nil {
					slog.ErrorContext(ctx, "can't set notification about time difference", err)
				}
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
				SubKind: types.NotificationDefaultWarningSubKind,
				Spec:    &notificationsv1.NotificationSpec{},
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
func generateNotificationMessage(inventories []inventoryMonitor) string {
	var messages []string
	for _, inv := range inventories {
		messages = append(messages, inv.String())
	}

	return "Incorrect system clock detected in the cluster, which may lead to certificate validation issues.\n" +
		"Ensure that the clock is accurate on all nodes to avoid potential access problems.\n" +
		"List of servers with a time shift: \n" + strings.Join(messages, "\n")
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
