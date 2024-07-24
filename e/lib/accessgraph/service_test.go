/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package accessgraph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func TestService_Query(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	env := setup(t)

	env.createUsersAndRoles(t, ctx)

	t.Run("Query is not allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user-no-perm", []string{}, map[string][]string{})

		_, err := env.service.Query(ctx, &accessgraphv1alpha.QueryRequest{})
		require.ErrorContains(t, err, "not allowed to read the access graph")

		require.Equal(t, 0, env.fakeAccessGraphServer.calledFunctions["Query"])
	})

	t.Run("Query is allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user", []string{"test-role"}, map[string][]string{})

		_, err := env.service.Query(ctx, &accessgraphv1alpha.QueryRequest{})
		require.NoError(t, err)

		require.Equal(t, 1, env.fakeAccessGraphServer.calledFunctions["Query"])
	})

	t.Run("GetFile is always allowed", func(t *testing.T) {
		ctx = genUserContext(ctx, "test-user-no-perm", []string{}, map[string][]string{})

		_, err := env.service.GetFile(ctx, &accessgraphv1alpha.GetFileRequest{})
		require.NoError(t, err)
	})
}
