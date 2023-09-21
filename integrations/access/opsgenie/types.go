/*
Copyright 2015-2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

// Responder represents an Opsgenie responder
type Responder struct {
	// Name is the name of the responder.
	Name string `json:"name,omitempty"`
	// Username is the opsgenie username of the responder.
	Username string `json:"username,omitempty"`
	// Type is the type of responder team/user
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
	Alert Alert `json:"alert"`
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
