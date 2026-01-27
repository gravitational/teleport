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

package services

import (
	"slices"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

// Preconditions is a set of decisionpb.PreconditionKind. Enforcement of preconditions should be done in the order
// returned by the Sorted method for consistency. If multiple preconditions are not satisfied, a combined error that
// lists all unsatisfied preconditions should be returned along with any relevant details.
type Preconditions map[decisionpb.PreconditionKind]struct{}

// Sorted returns the kinds in sorted order.
func (p Preconditions) Sorted() []decisionpb.PreconditionKind {
	kinds := make([]decisionpb.PreconditionKind, 0, len(p))

	for kind := range p {
		kinds = append(kinds, kind)
	}

	slices.Sort(kinds)

	return kinds
}
