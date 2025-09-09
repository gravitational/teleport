// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package scimsdk

// GroupMember holds a single group member
type GroupMember struct {
	// ExternalID is the downstream system's ID for the list member.
	// TODO: rename to ID, as in SCIM terms this is the ID, not the ExternalID.
	ExternalID string `json:"value,omitempty"`
	// Display is the display name of the member
	Display string `json:"display,omitempty"`
	// Ref is the reference to the member
	Ref string `json:"$ref,omitempty"`
	// Type is the type of the member
	Type string `json:"type,omitempty"`
}

// Group represents a SCIM Group resource.
type Group struct {
	// ID is a unique identifier for a Group as defined by the Service Provider.
	ID string `json:"id,omitempty"`
	// Meta contains resource metadata
	Meta *Metadata `json:"meta,omitempty"`
	// Schemas is a list of URIs that are used to indicate the namespaces of the SCIM schemas used for the representation of a resource.
	Schemas []string `json:"schemas,omitempty"`
	// DisplayName is the name of the Group, suitable for display to end-users.
	DisplayName string `json:"displayName,omitempty"`
	// Members is a list of members of the Group
	Members []*GroupMember `json:"members,omitempty"`
}

// ListGroupResponse represents a SCIM Group list response.
type ListGroupResponse struct {
	// Schemas is a list of URIs that are used to indicate the namespaces of the SCIM schemas used for the representation of a resource.
	Schemas []string `json:"schemas"`
	// TotalResults is the total number of results returned by the list or query operation.
	TotalResults int32 `json:"totalResults"`
	// StartIndex is the 1-based index of the first result in the current set of list results.
	StartIndex int32 `json:"startIndex"`
	// ItemsPerPage is the number of resources returned in a list response page.
	ItemsPerPage int32 `json:"itemsPerPage"`
	// Groups is a list of Group resources.
	Groups []*Group `json:"Resources"`
}
