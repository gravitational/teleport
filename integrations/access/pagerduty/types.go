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

package pagerduty

// PagerDuty API types

type PaginationQuery struct {
	Limit  uint `url:"limit,omitempty"`
	Offset uint `url:"offset,omitempty"`
	Total  bool `url:"total,omitempty"`
}

type PaginationResult struct {
	Limit  uint `json:"limit"`
	Offset uint `json:"offset"`
	More   bool `json:"more"`
	Total  uint `json:"total"`
}

type ErrorResult struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

type Reference struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
}

type Details struct {
	Type    string `json:"type,omitempty"`
	Details string `json:"details,omitempty"`
}

type ExtensionSchema struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type ListExtensionSchemasResult struct {
	PaginationResult
	ExtensionSchemas []ExtensionSchema `json:"extension_schemas"`
}

type Extension struct {
	ID               string      `json:"id,omitempty"`
	Name             string      `json:"name"`
	EndpointURL      string      `json:"endpoint_url"`
	ExtensionObjects []Reference `json:"extension_objects"`
	ExtensionSchema  Reference   `json:"extension_schema"`
}

type ExtensionBody struct {
	Name             string      `json:"name"`
	EndpointURL      string      `json:"endpoint_url"`
	ExtensionObjects []Reference `json:"extension_objects"`
	ExtensionSchema  Reference   `json:"extension_schema"`
}

type ExtensionBodyWrap struct {
	Extension ExtensionBody `json:"extension"`
}

type ExtensionResult struct {
	Extension Extension `json:"extension"`
}

type ListExtensionsResult struct {
	PaginationResult
	Extensions []Extension `json:"extensions"`
}

type Service struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	EscalationPolicy Reference `json:"escalation_policy"`
}

type ServiceResult struct {
	Service Service `json:"service"`
}

type ListServicesQuery struct {
	PaginationQuery
	Query string `url:"query,omitempty"`
}

type ListServicesResult struct {
	PaginationResult
	Services []Service `json:"services"`
}

type Incident struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Status      string               `json:"status"`
	IncidentKey string               `json:"incident_key"`
	Service     Reference            `json:"service"`
	Assignments []IncidentAssignment `json:"assignments"`
	Body        Details              `json:"body"`
}

type IncidentAssignment struct {
	At       string    `json:"at"`
	Assignee Reference `json:"assignee"`
}

type IncidentBody struct {
	ID          string    `json:"id,omitempty"`
	Title       string    `json:"title,omitempty"`
	IncidentKey string    `json:"incident_key,omitempty"`
	Service     Reference `json:"service"`
	Body        Details   `json:"body"`
	Type        string    `json:"type,omitempty"`
	Status      string    `json:"status,omitempty"`
}

type IncidentBodyWrap struct {
	Incident IncidentBody `json:"incident"`
}

type IncidentResult struct {
	Incident Incident `json:"incident"`
}

type ListIncidentsResult struct {
	PaginationResult
	Incidents []Incident `json:"incidents"`
}

type IncidentNote struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type IncidentNoteBody struct {
	Content string `json:"content,omitempty"`
}

type IncidentNoteBodyWrap struct {
	Note IncidentNoteBody `json:"note"`
}

type IncidentNoteResult struct {
	Note IncidentNote `json:"note"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type UserResult struct {
	User User `json:"user"`
}

type ListUsersQuery struct {
	PaginationQuery
	Query string `url:"query,omitempty"`
}

type ListUsersResult struct {
	PaginationResult
	Users []User `json:"users"`
}

type OnCall struct {
	User             Reference `json:"user"`
	EscalationPolicy Reference `json:"escalation_policy"`
}

type ListOnCallsQuery struct {
	PaginationQuery
	UserIDs             []string `url:"user_ids,omitempty,brackets"`
	EscalationPolicyIDs []string `url:"escalation_policy_ids,omitempty,brackets"`
}

type ListOnCallsResult struct {
	PaginationResult
	OnCalls []OnCall `json:"oncalls"`
}

// ListAbilitiesResult describes the output of the PagerDuty /abilities
// API call.
type ListAbilitiesResult struct {
	Abilities []string `json:"abilities"`
}
