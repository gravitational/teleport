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

package cache

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestSingletonStore(t *testing.T) {
	store := singletonStore[types.StaticTokens]{}

	spec := types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Token:   "static1",
				Roles:   types.SystemRoles{types.RoleAuth, types.RoleNode},
				Expires: time.Now().UTC().Add(time.Hour),
			},
		},
	}
	staticTokens, err := types.NewStaticTokens(spec)
	require.NoError(t, err)

	out, err := store.get()
	require.ErrorIs(t, err, &trace.NotFoundError{Message: "no value for singleton of type *types.StaticTokens"})
	require.Zero(t, out)

	require.NoError(t, store.put(staticTokens))

	out, err = store.get()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(staticTokens, out))

	require.NoError(t, store.delete(staticTokens))

	out, err = store.get()
	require.ErrorIs(t, err, &trace.NotFoundError{Message: "no value for singleton of type *types.StaticTokens"})
	require.Zero(t, out)

	st := &types.StaticTokensV2{Spec: spec}
	require.NoError(t, store.put(st))

	out, err = store.get()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(st, out))

	require.NoError(t, store.clear())

	out, err = store.get()
	require.ErrorIs(t, err, &trace.NotFoundError{Message: "no value for singleton of type *types.StaticTokens"})
	var empty types.StaticTokens
	require.Empty(t, cmp.Diff(empty, out))
}
