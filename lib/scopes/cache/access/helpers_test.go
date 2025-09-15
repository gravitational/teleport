/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package access

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache/assignments"
	"github.com/gravitational/teleport/lib/scopes/cache/roles"
)

func TestStreamRoles(t *testing.T) {
	const roleCount = 503

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upstream := roles.NewRoleCache()

	expectedRoleNames := make([]string, 0, roleCount)

	for i := range roleCount {
		name := fmt.Sprintf("role-%d", i)
		err := upstream.Put(&scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Scope: "/foo",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/foo"},
			},
			Version: types.V1,
		})
		require.NoError(t, err)
		expectedRoleNames = append(expectedRoleNames, name)
	}

	gotRoleNames := make([]string, 0, roleCount)

	for role, err := range StreamRoles(ctx, upstream) {
		require.NoError(t, err)
		require.NotNil(t, role)

		gotRoleNames = append(gotRoleNames, role.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedRoleNames, gotRoleNames)
}

func TestStreamAssignments(t *testing.T) {
	const assignmentCount = 503

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upstream := assignments.NewAssignmentCache()

	expectedAssignmentNames := make([]string, 0, assignmentCount)

	for range assignmentCount {
		name := uuid.New().String()
		err := upstream.Put(&scopedaccessv1.ScopedRoleAssignment{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "some-role",
						Scope: "/foo",
					},
				},
			},
			Version: types.V1,
		})
		require.NoError(t, err)
		expectedAssignmentNames = append(expectedAssignmentNames, name)
	}

	gotAssignmentNames := make([]string, 0, assignmentCount)

	for assignment, err := range StreamAssignments(ctx, upstream) {
		require.NoError(t, err)
		require.NotNil(t, assignment)

		gotAssignmentNames = append(gotAssignmentNames, assignment.GetMetadata().GetName())
	}

	require.ElementsMatch(t, expectedAssignmentNames, gotAssignmentNames)
}
