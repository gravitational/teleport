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

package ui

import (
	"time"

	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ui"
)

type Notification struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	SubKind     string     `json:"subKind"`
	Created     time.Time  `json:"created"`
	Clicked     bool       `json:"clicked"`
	TextContent string     `json:"textContent,omitempty"`
	Labels      []ui.Label `json:"labels"`
}

// MakeNotification creates a notification object for the WebUI.
func MakeNotification(notification *notificationsv1.Notification) Notification {
	labels := ui.MakeLabelsWithoutInternalPrefixes(notification.Metadata.Labels)

	clicked := notification.Metadata.GetLabels()[types.NotificationClickedLabel] == "true"

	return Notification{
		ID:          notification.Metadata.GetName(),
		Title:       notification.Metadata.GetLabels()[types.NotificationTitleLabel],
		SubKind:     notification.SubKind,
		Created:     notification.Spec.Created.AsTime(),
		Clicked:     clicked,
		TextContent: notification.Metadata.GetLabels()[types.NotificationTextContentLabel],
		Labels:      labels,
	}
}
