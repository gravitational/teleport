/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
package sshca

import (
	"encoding/json"

	"github.com/gravitational/trace"
)

// requestIDs is a collection of IDs for privilege escalation requests. This type is used
// for serialization/deserialization of request IDs for embedding in certificates. Most usecases
// should prefer representing request IDs as a slice of strings.
type requestIDs struct {
	AccessRequests []string `json:"access_requests,omitempty"`
}

// Marshal encodes requestIDs into the canonical JSON format expected for teleport certificates.
func (r *requestIDs) Marshal() ([]byte, error) {
	data, err := json.Marshal(r)
	return data, trace.Wrap(err)
}

// Unmarshal decodes requestIDs from the canonical JSON format expected for teleport certificates.
func (r *requestIDs) Unmarshal(data []byte) error {
	return trace.Wrap(json.Unmarshal(data, r))
}

// IsEmpty returns true if the requestIDs is empty.
func (r *requestIDs) IsEmpty() bool {
	return len(r.AccessRequests) < 1
}

// certRoles is a helper for marshaling and unmarshaling roles list to the format
// used by the teleport ssh certificate role extension.
type certRoles struct {
	// Roles is a list of roles
	Roles []string `json:"roles"`
}

// Marshal marshals roles to the format used by the teleport ssh certificate role extension.
func (r *certRoles) Marshal() ([]byte, error) {
	data, err := json.Marshal(r)
	return data, trace.Wrap(err)
}

// Unmarshal unmarshals roles from the teleport ssh certificate role extension format.
func (r *certRoles) Unmarshal(data []byte) error {
	return trace.Wrap(json.Unmarshal(data, r))
}
