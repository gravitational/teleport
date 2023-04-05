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
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	AlertKey string  `json:"alert_key"`
	Body     Details `json:"body"`
}

// AlertBody represents and Opsgenie alert body
type AlertBody struct {
	Message     string      `json:"message,omitempty"`
	Alias       string      `json:"alias,omitempty"`
	Description string      `json:"description,omitempty"`
	Responders  []Responder `json:"responders,omitempty"`
	Priority    string      `json:"priority,omitempty"`
}

// Responder represents an Opsgenie responder
type Responder struct {
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Type     string `json:"type,omitempty"`
	ID       string `json:"id,omitempty"`
}

// RespondersResult represents a group of Opsgenie responders
type RespondersResult struct {
	Data struct {
		OnCallRecipients []string `json:"onCallRecipients,omitempty"`
	} `json:"data,omitempty"`
}

// AlertResult is a wrapper around Alert
type AlertResult struct {
	Alert Alert `json:"alert"`
}

// AlertNote represents an Opsgenie alert note
type AlertNote struct {
	User   string `json:"user"`
	Source string `json:"source"`
	Note   string `json:"note"`
}

// Details represents the details field of an Opsgenie alert
type Details struct {
	Type    string `json:"type,omitempty"`
	Details string `json:"details,omitempty"`
}
