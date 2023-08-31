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

package servicenow

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// PluginData is a data associated with access request that we store in Teleport using UpdatePluginData API.
type PluginData struct {
	RequestData
	ServicenowData
}

// ServicenowData is the data associated with access request that we store in Teleport using UpdatePluginData API.
type ServicenowData struct {
	// IncidentID is the servicenow sys_id of the incident
	IncidentID string
}

// Incident represents a servicenow incident.
type Incident struct {
	// IncidentID is the sys_id of the incident
	IncidentID string `json:"sys_id,omitempty"`
	// ShortDescription contains a brief summary of the incident.
	ShortDescription string `json:"short_description,omitempty"`
	// Description contains the description of the incident.
	Description string `json:"description,omitempty"`
	// CloseCode contains the close code of the incident once it is resolved.
	CloseCode string `json:"close_code,omitempty"`
	// CloseNotes contains the closing comments on the incident once it is resolved.
	CloseNotes string `json:"close_notes,omitempty"`
	// IncidentState contains the current state the incident is in.
	IncidentState string `json:"incident_state,omitempty"`
	// WorkNotes contains comments on the progress of the incident.
	WorkNotes string `json:"work_notes,omitempty"`
}

const (
	ResolutionStateResolved = "6"
	ResolutionStateClosed   = "7"
)

// Resolution stores the resolution state and the servicenow close code.
type Resolution struct {
	// State is the state of the servicenow incident
	State string
	// CloseCode is the close code of the servicenow incident.
	CloseCode string
	// Reason is the reason the incident is being closed.
	Reason string
}

// RequestData stores a slice of some request fields in a convenient format.
type RequestData struct {
	// User is the requesting user.
	User string
	// Roles are the roles being requested.
	Roles []string
	// Created is the request creation timestamp.
	Created time.Time
	// RequestReason is the reason for the request.
	RequestReason string
	// ReviewCount is the number of the of the reviews on the access request.
	ReviewsCount int
	// Resolution is the final resolution of the access request.
	Resolution Resolution
	// SystemAnnotations contains key value annotations for the request.
	SystemAnnotations types.Labels
}

type onCallResult struct {
	Result []struct {
		// UserID is the ID of the on-call user.
		UserID string `json:"userId"`
	} `json:"result"`
}

type userResult struct {
	Result []struct {
		// Email is the email address in servicenow of the requested user.
		Email string `json:"email"`
	} `json:"result"`
}

type incidentResult struct {
	Result Incident `json:"result"`
}
