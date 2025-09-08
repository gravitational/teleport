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

package access

import (
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestEmptyRoleConverts verifies that a scoped role that is empty except for
// scoping/metadata converts to a reasonable default/unprivileged classic role.
func TestEmptyRoleConverts(t *testing.T) {
	t.Parallel()

	role, err := ScopedRoleToRole(&scopedaccessv1.ScopedRole{
		Kind: KindScopedRole,
		Metadata: &headerv1.Metadata{
			Name: "test",
		},
		Scope: "/foo",
		Spec: &scopedaccessv1.ScopedRoleSpec{
			AssignableScopes: []string{"/foo/bar"},
		},
		Version: types.V1,
	}, "/foo/bar")
	require.NoError(t, err)
	require.NotNil(t, role)
	require.Equal(t, "test@/foo/bar", role.GetName())
}
