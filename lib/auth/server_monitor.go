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
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/services"
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
	// systemClockNotificationExpiration is the expiration time for global notification
	// warning about time difference.
	systemClockNotificationExpiration = time.Hour * 24 * 30
	// systemClockMessagesLimit is limit for showing the list of affected inventories.
	systemClockMessagesLimit = 10
)

// MonitorSystemTime runs the periodic check for iterating through all inventories
// to ping them and receive the system clock difference.
func (a *Server) MonitorSystemTime(ctx context.Context) error {
	checkInterval := interval.New(interval.Config{
		FirstDuration: time.Second * 10,
		Duration:      systemClockCheckCycle,
		Clock:         a.GetClock(),
	})
	defer checkInterval.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-checkInterval.Next():
			a.checkInventorySystemClocks(ctx)
		}
	}
}

// checkInventoryClocks iterates through inventory store instance state to gather
// information about the system clock differences.
func (a *Server) checkInventorySystemClocks(ctx context.Context) {
	var counter int
	var messages []string
	a.inventory.Iter(func(handle inventory.UpstreamHandle) {
		hello := handle.Hello()
		handle.VisitInstanceState(func(ref inventory.InstanceStateRef) (update inventory.InstanceStateUpdate) {
			if ref.LastHeartbeat != nil && ref.LastHeartbeat.GetLastMeasurement() != nil {
				m := ref.LastHeartbeat.GetLastMeasurement()
				// RequestDuration is request and response duration between upstream and downstream,
				// since we capture system clock on downstream we have to ignore response duration
				// and only count request duration.
				diff := m.ControllerSystemClock.Sub(m.SystemClock) - m.RequestDuration/2
				if diff > systemClockThreshold || -diff > systemClockThreshold {
					slog.WarnContext(ctx, "server time difference detected",
						"server", hello.GetServerID(),
						"services", hello.GetServices(),
						"difference", durationText(diff),
					)
					if counter < systemClockMessagesLimit {
						messages = append(messages, fmt.Sprintf(
							"%s[%s] is %s",
							hello.GetServerID(),
							types.SystemRoles(hello.GetServices()).String(),
							durationText(diff),
						))
					}
					counter++
				}
			}
			return
		})
	})

	if len(messages) > 0 {
		title, text := generateClockWarningNotificationMessage(messages, counter)
		err := upsertClockWarningGlobalNotification(ctx, a.Services, title, text)
		if err != nil {
			slog.ErrorContext(ctx, "can't set notification about system clock issue", "error", err)
		}
	}
}

// upsertClockWarningGlobalNotification sets predefined global notification for notifying the issues with the cluster
// servers related to the system clock difference in nodes.
func upsertClockWarningGlobalNotification(ctx context.Context, notification services.Notifications, title, text string) error {
	now := time.Now()
	_, err := notification.UpsertGlobalNotification(ctx, &notificationsv1.GlobalNotification{
		Kind:     types.KindGlobalNotification,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: systemClockNotificationWarningName},
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_All{
				All: true,
			},
			Notification: &notificationsv1.Notification{
				SubKind: types.NotificationDefaultWarningSubKind,
				Spec: &notificationsv1.NotificationSpec{
					Created: timestamppb.New(now),
				},
				Metadata: &headerv1.Metadata{
					Expires: timestamppb.New(now.Add(systemClockNotificationExpiration)),
					Name:    systemClockNotificationWarningName,
					Labels: map[string]string{
						types.NotificationTitleLabel:       title,
						types.NotificationTextContentLabel: text,
					},
				},
			},
		},
	})
	return trace.Wrap(err)
}

// generateClockWarningNotificationMessage formats the notification message with the inventory list.
func generateClockWarningNotificationMessage(messages []string, total int) (string, string) {
	title := "Incorrect system clock detected in the cluster"
	text := "Incorrect system clock may lead to certificate validation issues.\n" +
		"Ensure that the clock is accurate on all nodes to avoid potential access problems.\n" +
		"All comparisons are made with the Auth service system clock.\n" +
		"List of servers with a time drift: \n" + strings.Join(messages, ", ")

	if total > len(messages) {
		text += fmt.Sprintf("(%d in total)", total)
	}

	return title, text
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
