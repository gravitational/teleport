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

import (
	"time"
)

const (
	// PatchOpSchema is the SCIM schema name for the PatchOp object
	// as per RFC-7644 Section 3.5.2
	PatchOpSchema = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
)

const (
	// ContentType is the MIME type for SCIM resources
	ContentType = "application/scim+json"
	// ContentTypeHeader is the HTTP header name for the SCIM content type.
	ContentTypeHeader = "Content-Type"
)

const (
	// AttributeID is the SCIM attribute name for the ID field
	AttributeID = "id"
	// AttributeExternalID is the SCIM attribute name for the ExternalID field
	AttributeExternalID = "externalId"
	// AttributeSchemas is the SCIM attribute name for the Schemas field
	AttributeSchemas = "schemas"
	// AttributeMeta is the SCIM attribute name for the Meta field
	AttributeMeta = "meta"
)

var reservedAttributeNames = [...]string{
	AttributeID,
	AttributeExternalID,
	AttributeSchemas,
	AttributeMeta,
}

const (
	// OpAdd is the SCIM Patch operation name for adding a new attribute
	OpAdd = "add"
	// OpReplace is the SCIM Patch operation name for replacing an existing attribute
	OpReplace = "replace"
	// OpRemove is the SCIM Patch operation name for removing an existing attribute
	OpRemove = "remove"
)

// Metadata encodes the JSON wire format of the SCIM resource metadata.
type Metadata struct {
	ResourceType string     `json:"resourceType" mapstructure:"resourceType,omitempty"`
	Created      *time.Time `json:"created,omitempty" mapstructure:"created,omitempty"`
	LastModified *time.Time `json:"lastModified,omitempty" mapstructure:"lastModified,omitempty"`
	Location     string     `json:"location,omitempty" mapstructure:"location,omitempty"`
	Version      string     `json:"version,omitempty" mapstructure:"version,omitempty"`
}

// AttributeSet is an arbitrary mapping on names to structured values. Used as
// an intermediary format for parsing and formatting SCIM resources
type AttributeSet map[string]any

// Resource represents the JSON wire format of a SCIM Resource, which is
// essentially some metadata with a trailing collection of arbitrarily
// structured attributes
type Resource struct {
	Schemas    []string  `json:"schemas" mapstructure:"schemas,omitempty"`
	ID         string    `json:"id,omitempty" mapstructure:"id,omitempty"`
	ExternalID string    `json:"externalId,omitempty" mapstructure:"externalId,omitempty"`
	Meta       *Metadata `json:"meta,omitempty" mapstructure:"meta,omitempty"`

	Attributes AttributeSet `json:"-" mapstructure:",remain,omitempty"`
}

// PatchOp represents a single operation in a SCIM Patch request
type PatchOp struct {
	Operation string `json:"op"`
	Path      string `json:"path"`
	Value     any    `json:"value"`
}

// MultiPatchOp represents a single operation in a SCIM MultiPatch request
type MultiPatchOp struct {
	Operation string         `json:"op"`
	Value     map[string]any `json:"value"`
}

// ListResponse is the JSON wire format of a SCIM list response
type ListResponse struct {
	Schemas      []string       `json:"schemas"`
	TotalResults int32          `json:"totalResults"`
	StartIndex   int32          `json:"startIndex"`
	ItemsPerPage int32          `json:"itemsPerPage"`
	Resources    []AttributeSet `json:"Resources"`
}

type PatchOperations struct {
	Schemas    []string  `json:"schemas,omitempty"`
	Operations []PatchOp `json:"Operations,omitempty"`
}
