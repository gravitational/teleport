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
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	// systemClockCheckCycle is the period when system clock comparison is launched
	// across all inventories, to be gathered for global notifications, if any.
	systemClockCheckCycle = 10 * time.Minute
	// systemClockThreshold is the duration threshold for triggering a warning
	// if the time difference exceeds this threshold.
	systemClockThreshold = time.Minute
	// systemClockNotificationWarningName is the ID for adding the global notification.
	systemClockNotificationWarningName = "cluster-monitor-system-clock-warning"
)

// MonitorSystemTime runs the periodic check for iterating through all inventories
// to ping them and receive the system clock difference.
func (a *Server) MonitorSystemTime(ctx context.Context) error {
	checkInterval := interval.New(interval.Config{
		FirstDuration: time.Minute,
		Duration:      systemClockCheckCycle,
		Clock:         a.GetClock(),
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer checkInterval.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-checkInterval.Next():
			var inventories []string
			a.inventory.Iter(func(handle inventory.UpstreamHandle) {
				info := handle.Hello()
				diff, err := handle.TimeReconciliation(ctx, insecurerand.Uint64())
				if err != nil {
					slog.ErrorContext(ctx, "error getting time reconciliation")
					return
				}

				if (diff > 0 && diff > systemClockThreshold) || (diff < 0 && -diff > systemClockThreshold) {
					inventories = append(inventories, fmt.Sprintf(
						" - %s[%s] is %s",
						info.GetServerID(),
						types.SystemRoles(info.GetServices()).String(),
						durationText(diff),
					))
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
					slog.ErrorContext(ctx, "can't set notification about system clock issue", "error", err)
					continue
				}
			}
		}
	}
}

// upsertGlobalNotification sets predefined global notification for notifying the issues with the cluster
// servers related to the system clock difference in nodes.
func upsertGlobalNotification(ctx context.Context, services *Services, text string) error {
	_, err := services.UpsertGlobalNotification(ctx, &notificationsv1.GlobalNotification{
		Kind:     types.KindGlobalNotification,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: systemClockNotificationWarningName},
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_All{
				All: true,
			},
			Notification: &notificationsv1.Notification{
				SubKind: types.NotificationDefaultWarningSubKind,
				Spec:    &notificationsv1.NotificationSpec{},
				Metadata: &headerv1.Metadata{
					Name: systemClockNotificationWarningName,
					Labels: map[string]string{
						types.NotificationTitleLabel: text,
					},
				},
			},
		},
	})
	return trace.Wrap(err)
}

// generateNotificationMessage formats the notification message with the inventory list.
func generateNotificationMessage(messages []string) string {
	var more string
	if len(messages) > 10 {
		more = fmt.Sprintf("Only 10 servers are shown out of the list of %d.", len(messages))
		messages = messages[:10]
	}
	return "Incorrect system clock detected in the cluster, which may lead to certificate validation issues.\n" +
		"Ensure that the clock is accurate on all nodes to avoid potential access problems.\n" +
		"All comparisons are made with the Auth service system clock." +
		"List of servers with a time drift: \n" + strings.Join(messages, "\n") + more
}

// durationText formats the specified duration to text by adding the suffix "ahead" or "behind"
// and converts nanoseconds to a formatted text with hours, minutes and seconds.
func durationText(duration time.Duration) string {
	if duration > 0 {
		return fmt.Sprintf("%s ahead", duration.String())
	} else {
		return fmt.Sprintf("%s behind", (-duration).String())
	}
}
