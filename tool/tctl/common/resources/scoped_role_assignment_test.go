/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"testing"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

func TestScopedRoleAssignmentCollection_WriteText(t *testing.T) {
	assignments := []*scopedaccessv1.ScopedRoleAssignment{
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:     scopedaccess.KindScopedRoleAssignment,
			SubKind:  scopedaccess.SubKindDynamic,
			Version:  types.V1,
			Metadata: headerv1.Metadata_builder{Name: "alice-assignment"}.Build(),
			Scope:    "/staging",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "/staging::role1",
						Scope: "/staging/west",
					}.Build(),
				},
			}.Build(),
		}.Build(),
		scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:     scopedaccess.KindScopedRoleAssignment,
			SubKind:  scopedaccess.SubKindDynamic,
			Version:  types.V1,
			Metadata: headerv1.Metadata_builder{Name: "mybot-assignment"}.Build(),
			Scope:    "/staging",
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				Bot: "/staging/west::mybot",
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  "/staging::role1",
						Scope: "/staging/west",
					}.Build(),
					scopedaccessv1.Assignment_builder{
						Role:  "/staging::role2",
						Scope: "/staging/west",
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}

	table := asciitable.MakeTable(
		[]string{"SubKind", "ID", "Assignee Type", "Assignee", "Assigns"},
		[]string{"dynamic", "/staging::alice-assignment", "user", "alice", "/staging::role1 -> /staging/west"},
		[]string{"dynamic", "/staging::mybot-assignment", "bot", "/staging/west::mybot", "/staging::role1 -> /staging/west, /staging::role2 -> /staging/west"},
	)

	formatted := table.AsBuffer().String()

	collectionFormatTest(t, NewScopedRoleAssignmentCollection(assignments), formatted, formatted)
}
