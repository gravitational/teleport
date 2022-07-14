/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/lite"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestSystemRoleAssertions(t *testing.T) {
	const serverID = "test-server"
	const assertionID = "test-assertion"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lite, err := lite.NewWithConfig(ctx, lite.Config{Path: t.TempDir()})
	require.NoError(t, err)

	defer lite.Close()

	unstable := NewUnstableService(lite)

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
		err = unstable.AssertSystemRole(ctx, proto.UnstableSystemRoleAssertion{
			ServerID:    serverID,
			AssertionID: assertionID,
			SystemRole:  role,
		})
		require.NoError(t, err)

		assertions, err := unstable.GetSystemRoleAssertions(ctx, serverID, assertionID)
		require.NoError(t, err)

		require.Equal(t, len(expect), len(assertions.SystemRoles))

		for _, r := range assertions.SystemRoles {
			_, ok := expect[r]
			require.True(t, ok)
		}
	}
}
