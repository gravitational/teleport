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

package accesslists

import (
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/scopes"
)

// NormalizedSQN is a scope-qualified name that has been normalized for
// comparisons and usage as a map key.
type NormalizedSQN struct {
	// Scope is the resource's normalized scope path, e.g. "/staging/west".
	Scope string
	// Name is the resource's name within its scope, e.g. "mylist".
	Name string
}

// NormalizeSQN returns a scope-qualified name that has been normalized for
// comparisons and usage as a map key.
func NormalizeSQN(sqn scopes.QualifiedName) NormalizedSQN {
	return NormalizedSQN{
		Scope: scopes.NormalizeForEquality(sqn.Scope),
		Name:  sqn.Name,
	}
}

// ToScopesQualifiedName converts the NormalizedSQN to a [scopes.QualifiedName].
func (n NormalizedSQN) ToScopesQualifiedName() scopes.QualifiedName {
	return scopes.QualifiedName{
		Scope: n.Scope,
		Name:  n.Name,
	}
}

// ScopeQualifiedName returns the normalized scope-qualified name of the given access list.
func ScopeQualifiedName(list *accesslist.AccessList) NormalizedSQN {
	return NormalizeSQN(scopes.QualifiedName{
		Scope: list.Scope,
		Name:  list.Metadata.Name,
	})
}
