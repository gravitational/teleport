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

package model

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

			e := &embeddingRetrievalTool{
				nodeClient:        &mockNodeGetter{nodes: nodes},
				userAccessChecker: &mockAccessChecker{allowAccess: tt.hasAccess},
				currentUser:       testUser,
			}

			// Doing the real test
			ok, output, err := e.tryNodeLookupFromProxyCache(ctx)
			require.NoError(t, err)
			tt.assertLookupSuccessful(t, ok)
			require.Equal(t, tt.expectedOutput, output)
		})
	}
}
