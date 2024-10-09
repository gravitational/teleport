/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package datadog

// Datadog API types.
// See: https://docs.datadoghq.com/api/latest/

// Metadata contains metadata for all Datadog resources.
type Metadata struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
}

// PermissionsBody contains the response body for a list permissions request
//
// See: https://docs.datadoghq.com/api/latest/roles/#list-permissions
type PermissionsBody struct {
	Data []PermissionsData `json:"data,omitempty"`
}

// PermissionsData contains the permissions data.
type PermissionsData struct {
	Metadata
	Attributes PermissionsAttributes `json:"attributes,omitempty"`
}

// PermissionsAttributes contains the permissions attributes.
type PermissionsAttributes struct {
	Name       string `json:"name,omitempty"`
	Restricted bool   `json:"restricted"`
}

// IncidentBody contains the request/response body for an incident request.
//
// See: https://docs.datadoghq.com/api/latest/incidents
type IncidentsBody struct {
	Data IncidentsData `json:"data,omitempty"`
}

// IncidentData contains the incident data.
type IncidentsData struct {
	Metadata
	Attributes IncidentsAttributes `json:"attributes,omitempty"`
}

// IncidentsAttributes contains the incident attributes.
type IncidentsAttributes struct {
	Title               string               `json:"title,omitempty"`
	Fields              IncidentsFields      `json:"fields,omitempty"`
	NotificationHandles []NotificationHandle `json:"notification_handles,omitempty"`
}

// IncidentsFields contains the incident fields.
type IncidentsFields struct {
	Summary         *StringField      `json:"summary,omitempty"`
	Severity        *StringField      `json:"severity,omitempty"`
	State           *StringField      `json:"state,omitempty"`
	DetectionMethod *StringField      `json:"detection_method,omitempty"`
	RootCause       *StringField      `json:"root_cause,omitempty"`
	Teams           *StringSliceField `json:"teams,omitempty"`
	Services        *StringSliceField `json:"services,omitempty"`
}

// StringField represents a single string field value.
type StringField struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// StringSliceField represents a multi-value string field value.
type StringSliceField struct {
	Type  string   `json:"type,omitempty"`
	Value []string `json:"value,omitempty"`
}

// NotificationHandle represents an incident notification handle.
type NotificationHandle struct {
	DisplayName string `json:"display_name,omitempty"`
	Handle      string `json:"handle,omitempty"`
}

// TimelineBody contains the request/response body for an incident timeline request.
type TimelineBody struct {
	Data TimelineData `json:"data,omitempty"`
}

// TimelineData contains the incident timeline data.
type TimelineData struct {
	Metadata
	Attributes TimelineAttributes `json:"attributes,omitempty"`
}

// TimelineAttributes contains the incident timeline attributes.
type TimelineAttributes struct {
	CellType string          `json:"cell_type,omitempty"`
	Content  TimelineContent `json:"content,omitempty"`
}

// TimelineContent contains the incident tineline content.
type TimelineContent struct {
	Content string `json:"content,omitempty"`
}

type OncallTeamsBody struct {
	Data     []OncallTeamsData     `json:"data,omitempty"`
	Included []OncallTeamsIncluded `json:"included,omitempty"`
}

type OncallTeamsData struct {
	Metadata
	Attributes    OncallTeamsAttributes    `json:"attributes,omitempty"`
	Relationships OncallTeamsRelationships `json:"relationships,omitempty"`
}

type OncallTeamsAttributes struct {
	Name   string `json:"name,omitempty"`
	Handle string `json:"handle,omitempty"`
}

type OncallTeamsRelationships struct {
	OncallUsers OncallUsers `json:"oncall_users,omitempty"`
}

type OncallUsers struct {
	Data []OncallUsersData `json:"data,omitempty"`
}

type OncallUsersData struct {
	Metadata
}

type OncallTeamsIncluded struct {
	Metadata
	Attributes OncallTeamsIncludedAttributes `json:"attributes,omitempty"`
}

type OncallTeamsIncludedAttributes struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

type UsersBody struct {
	Data []UsersData `json:"data,omitempty"`
}

type UsersData struct {
	Metadata
	Attributes UsersAttributes `json:"attributes,omitempty"`
}

type UsersAttributes struct {
	Name     string `json:"name,omitempty"`
	Handle   string `json:"handle,omitempty"`
	Email    string `json:"email,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// ErrorResult contains the error response.
type ErrorResult struct {
	Errors []string `json:"errors"`
}
