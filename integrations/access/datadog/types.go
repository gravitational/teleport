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

// Datadog API types

type PermissionsBody struct {
	Data []PermissionData `json:"data,omitempty"`
}

type PermissionData struct {
	ID         string               `json:"id,omitempty"`
	Type       string               `json:"type,omitempty"`
	Attributes PermissionAttributes `json:"attributes,omitempty"`
}

type PermissionAttributes struct {
	Name       string `json:"name,omitempty"`
	Restricted bool   `json:"restricted"`
}

type IncidentBody struct {
	Data IncidentData `json:"data,omitempty"`
}

type IncidentData struct {
	ID         string             `json:"id,omitempty"`
	Type       string             `json:"type,omitempty"`
	Attributes IncidentAttributes `json:"attributes,omitempty"`
}

type IncidentAttributes struct {
	Title            string         `json:"title,omitempty"`
	CustomerImpacted bool           `json:"customer_impacted,omitempty"`
	Fields           IncidentFields `json:"fields,omitempty"`
}

type IncidentFields struct {
	Summary         *StringField      `json:"summary,omitempty"`
	Severity        *StringField      `json:"severity,omitempty"`
	State           *StringField      `json:"state,omitempty"`
	DetectionMethod *StringField      `json:"detection_method,omitempty"`
	RootCause       *StringField      `json:"root_cause,omitempty"`
	Teams           *StringSliceField `json:"teams,omitempty"`
	Services        *StringSliceField `json:"services,omitempty"`
}

type StringField struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type StringSliceField struct {
	Type  string   `json:"type,omitempty"`
	Value []string `json:"value,omitempty"`
}

type TimelineBody struct {
	Data TimelineData `json:"data,omitempty"`
}

type TimelineData struct {
	Type       string             `json:"type,omitempty"`
	Attributes TimelineAttributes `json:"attributes,omitempty"`
}

type TimelineAttributes struct {
	CellType string          `json:"cell_type,omitempty"`
	Content  TimelineContent `json:"content,omitempty"`
}

type TimelineContent struct {
	Content string `json:"content,omitempty"`
}

type ServiceBody struct {
	Data ServiceData `json:"data,omitempty"`
}

type ServiceData struct {
	Type       string            `json:"type,omitempty"`
	ID         string            `json:"id,omitempty"`
	Attributes ServiceAttributes `json:"attributes,omitempty"`
}

type ServiceAttributes struct {
	Schema ServiceSchema `json:"schema,omitempty"`
}

type ServiceSchema struct {
	DatadogService string `json:"dd-service,omitempty"`
	Team           string `json:"team,omitempty"`
}

type TeamsBody struct {
	Data []TeamData `json:"data,omitempty"`
}

type TeamData struct {
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type,omitempty"`
	Attributes TeamAttributes `json:"attributes,omitempty"`
}

type TeamAttributes struct {
	Name   string `json:"name,omitempty"`
	Handle string `json:"handle,omitempty"`
}

type OncallTeamsBody struct {
	Data     []OncallTeamData `json:"data,omitempty"`
	Included []OncallIncluded `json:"included,omitempty"`
}

type OncallTeamData struct {
	ID            string                  `json:"id,omitempty"`
	Type          string                  `json:"type,omitempty"`
	Attributes    OncallTeamAttributes    `json:"attributes,omitempty"`
	Relationships OncallTeamRelationships `json:"relationships,omitempty"`
}

type OncallTeamAttributes struct {
	Name   string `json:"name,omitempty"`
	Handle string `json:"handle,omitempty"`
}

type OncallTeamRelationships struct {
	OncallUsers OncallUsers `json:"oncall_users,omitempty"`
}

type OncallUsers struct {
	Data []OncallUsersData `json:"data,omitempty"`
}

type OncallUsersData struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
}

type OncallIncluded struct {
	ID         string                   `json:"id,omitempty"`
	Type       string                   `json:"type,omitempty"`
	Attributes OncallIncludedAttributes `json:"attributes,omitempty"`
}

type OncallIncludedAttributes struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

type UsersBody struct {
	Data []UserData `json:"data,omitempty"`
}

type UserData struct {
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type,omitempty"`
	Attributes UserAttributes `json:"attributes,omitempty"`
}

type UserAttributes struct {
	Name     string `json:"name,omitempty"`
	Handle   string `json:"handle,omitempty"`
	Email    string `json:"email,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type ErrorResult struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}
