/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package tools

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const testUser = "username"

// mockAccessChecker implements the services.AccessChecker and always validate
// or reject access based on its allowAccess field.
type mockAccessChecker struct {
	allowAccess bool
	services.AccessChecker
}

func (ac *mockAccessChecker) CheckAccess(_ services.AccessCheckable, _ services.AccessState, _ ...services.RoleMatcher) error {
	if ac.allowAccess {
		return nil
	}
	return trace.AccessDenied("user does not have access")
}

// mockNodeGetter returns a static list of nodes
type mockNodeGetter struct {
	nodes []types.Server
}

func (ng *mockNodeGetter) NodeCount() int {
	return len(ng.nodes)
}

func (ng *mockNodeGetter) GetNodes(_ context.Context, fn func(n services.Node) bool) []types.Server {
	var result []types.Server
	for _, node := range ng.nodes {
		if fn(node) {
			result = append(result, node)
		}
	}
	return result
}

func Test_embeddingRetrievalTool_tryNodeLookupFromProxyCache(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name                   string
		nodeCount              int
		hasAccess              bool
		assertLookupSuccessful require.BoolAssertionFunc
		expectedOutput         string
	}{
		{
			name:                   "No nodes",
			nodeCount:              0,
			hasAccess:              true,
			assertLookupSuccessful: require.False,
		},
		{
			name:                   "Few nodes",
			nodeCount:              2,
			hasAccess:              true,
			assertLookupSuccessful: require.True,
			expectedOutput: `name: node-0
kind: node
subkind: teleport
labels:
    foo: bar

name: node-1
kind: node
subkind: teleport
labels:
    foo: bar

`,
		},
		{
			name:                   "Few nodes without access",
			nodeCount:              2,
			hasAccess:              false,
			assertLookupSuccessful: require.False,
		},
		{
			name:                   "Too many nodes",
			nodeCount:              maxEmbeddingsPerLookup + 1,
			hasAccess:              true,
			assertLookupSuccessful: require.False,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup
			var err error
			nodes := make([]types.Server, tt.nodeCount)
			for i := 0; i < tt.nodeCount; i++ {
				nodeName := fmt.Sprintf("node-%d", i)
				nodes[i], err = types.NewServerWithLabels(nodeName, types.KindNode, types.ServerSpecV2{Hostname: nodeName}, map[string]string{"foo": "bar"})
				require.NoError(t, err)
			}

			toolCtx := &ToolContext{
				User:          testUser,
				AccessChecker: &mockAccessChecker{allowAccess: tt.hasAccess},
				NodeWatcher:   &mockNodeGetter{nodes: nodes},
			}

			// Doing the real test
			tool := EmbeddingRetrievalTool{}
			ok, output, err := tool.tryNodeLookupFromProxyCache(ctx, toolCtx)
			require.NoError(t, err)
			tt.assertLookupSuccessful(t, ok)
			require.Equal(t, tt.expectedOutput, output)
		})
	}
}
