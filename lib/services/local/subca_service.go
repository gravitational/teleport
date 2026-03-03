// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package local

import (
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// SubCAServiceParams holds creation parameters for [SubCAService].
type SubCAServiceParams struct {
	Backend backend.Backend
	Trust   services.AuthorityGetter
}

// SubCAService manages backend storage of CertAuthorityOverride resources.
//
// Follows RFD 153 / generic.Service semantics.
type SubCAService struct{}

// NewSubCAService creates a new service using the provided params.
func NewSubCAService(p SubCAServiceParams) (*SubCAService, error) {
	// TODO(codingllama): Validation, set fields.
	return &SubCAService{}, nil
}
