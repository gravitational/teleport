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

package opsgenie

// Alert represents an Opsgenie alert
type Alert struct {
	// ID is the id of the Opsgenie alert.
	ID string `json:"id"`
	// Title is the title of the Opsgenie alert.
	Title string `json:"title"`
	// Status is the curerent status of the Opsgenie alert.
	Status string `json:"status"`
	// AlertKey is the key of the Opsgenie alert.
	AlertKey string `json:"alert_key"`
	// Details are a map of key-value pairs to use as custom properties of the alert.
	Details map[string]string `json:"details"`
}

// AlertBody represents and Opsgenie alert body
type AlertBody struct {
	// Message is the message the alert is created with.
	Message string `json:"message,omitempty"`
	// Alias is the client-defined identifier of the alert.
	Alias string `json:"alias,omitempty"`
	// Description field of the alert.
	Description string `json:"description,omitempty"`
	// Responders are the teams/users that the alert will be routed to send notifications.
	Responders []Responder `json:"responders,omitempty"`
	// Priority is the priority the alert is created with.
	Priority string `json:"priority,omitempty"`
}

// Responder represents an Opsgenie responder.
// A responder is an entity that receives an alert.
// It can be a user, a team, or a schedule.
// The OpsGenie access plugin interacts with 2 types of responders:
// - it sends notifications to schedule responders
// - for auto-approval it looks up who the responders are for a given
// schedule and approves the request if a responder name matches the
// requester name.
type Responder struct {
	// Name is the name of the responder.
	Name string `json:"name,omitempty"`
	// Username is the opsgenie username of the responder.
	Username string `json:"username,omitempty"`
	// Type is the type of responder team/user/schedule.
	Type string `json:"type,omitempty"`
	// ID is the ID of the responder.
	ID string `json:"id,omitempty"`
}

// RespondersResult represents a group of Opsgenie responders
type RespondersResult struct {
	// Data is a wrapper around the OnCallRecipients.
	Data struct {
		OnCallRecipients []string `json:"onCallRecipients,omitempty"`
	} `json:"data,omitempty"`
}

// AlertResult is a wrapper around Alert
type AlertResult struct {
	// Alert contains the actual alert data.
	Alert Alert `json:"data"`
}

// AlertNote represents an Opsgenie alert note
type AlertNote struct {
	// User is the user that created the note.
	User string `json:"user"`
	// Source is the display name of the request source.
	Source string `json:"source"`
	// Note is the alert note.
	Note string `json:"note"`
}

// CreateAlertResult represents the resulting request information from an Opsgenie create alert request.
type CreateAlertResult struct {
	Result    string  `json:"result"`
	Took      float64 `json:"took"`
	RequestID string  `json:"requestId"`
}

// GetAlertRequestResult represents the response of a completed Opsgenie create alert request.
type GetAlertRequestResult struct {
	Data struct {
		AlertID string `json:"alertId"`
	} `json:"data"`
}
