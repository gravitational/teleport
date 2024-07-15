/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestSystemRoleAssertions(t *testing.T) {
	const serverID = "test-server"
	const assertionID = "test-assertion"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	defer backend.Close()

	assertion := NewAssertionReplayService(backend)
	unstable := NewUnstableService(backend, assertion)

	_, err = unstable.GetSystemRoleAssertions(ctx, serverID, assertionID)
	require.True(t, trace.IsNotFound(err))

	roles := []types.SystemRole{
		types.RoleNode,
		types.RoleAuth,
		types.RoleProxy,
	}

	expect := make(map[types.SystemRole]struct{})

	for _, role := range roles {
		expect[role] = struct{}{}
		err = unstable.AssertSystemRole(ctx, proto.SystemRoleAssertion{
			ServerID:    serverID,
			AssertionID: assertionID,
			SystemRole:  role,
		})
		require.NoError(t, err)

		assertions, err := unstable.GetSystemRoleAssertions(ctx, serverID, assertionID)
		require.NoError(t, err)

		require.Equal(t, len(expect), len(assertions.SystemRoles))
		require.Subset(t, expect, assertions.SystemRoles)
	}
}
