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

package servicenow

import (
	"time"
)

// PluginData is a data associated with access request that we store in Teleport using UpdatePluginData API.
type PluginData struct {
	RequestData
	ServiceNowData
}

// ServiceNowData is the data associated with access request that we store in Teleport using UpdatePluginData API.
type ServiceNowData struct {
	// IncidentID is the serviceNow sys_id of the incident
	IncidentID string
}

// Incident represents a serviceNow incident.
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
	// Caller is the user on whose behalf the incident is being created. (Must be an existing servicenow user)
	Caller string `json:"caller_id,omitempty"`
	// AssignedTo is the ServiceNow user the incident is assigned.
	AssignedTo string `json:"assigned_to,omitempty"`
}

const (
	// ServiceNow uses a value of 1-8 to indicate incident state
	// https://support.servicenow.com/kb?id=kb_article_view&sysparm_article=KB0564465

	// ResolutionStateResolved is the incident state for a resolved incident
	ResolutionStateResolved = "6"
	// ResolutionStateClosed is the incident state for a closed incident
	ResolutionStateClosed = "7"
)

// Resolution stores the resolution state and the servicenow close code.
type Resolution struct {
	// State is the state of the servicenow incident
	State string
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
	SystemAnnotations map[string][]string
	// Resources are the resources being requested.
	Resources []string
	// SuggestedReviewers are the suggested reviewers for this access request.
	SuggestedReviewers []string
}

// OnCallResult represents the response returned from a whoisoncall request to ServiceNow.
type OnCallResult struct {
	Result []struct {
		// UserID is the ID of the on-call user.
		UserID string `json:"userId"`
	} `json:"result"`
}

// UserResult represents the response returned when retieving a user from ServiceNow.
type UserResult struct {
	Result struct {
		// UserName is the username in servicenow of the requested user.
		// username chosen over email as identifier as it is guaranteed to be set.
		UserName string `json:"user_name"`
	} `json:"result"`
}

// IncidentResult represents the response returned when retieving an incident from ServiceNow.
type IncidentResult struct {
	Result struct {
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
	} `json:"result"`
}

// errorResult represents the error response returned from ServiceNow.
type errorResult struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
