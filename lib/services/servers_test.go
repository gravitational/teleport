/*
Copyright 2015 Gravitational, Inc.

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

package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestServersCompare tests comparing two servers
func TestServersCompare(t *testing.T) {
	t.Parallel()

	t.Run("compare servers", func(t *testing.T) {
		node := &types.ServerV2{
			Kind:    types.KindNode,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      "node1",
				Namespace: apidefaults.Namespace,
				Labels:    map[string]string{"a": "b"},
			},
			Spec: types.ServerSpecV2{
				Addr:      "localhost:3022",
				CmdLabels: map[string]types.CommandLabelV2{"a": {Period: types.Duration(time.Minute), Command: []string{"ls", "-l"}}},
				Version:   "4.0.0",
			},
		}
		node.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))
		// Server is equal to itself
		require.Equal(t, CompareServers(node, node), Equal)

		// Only timestamps are different
		node2 := *node
		node2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, CompareServers(node, &node2), OnlyTimestampsDifferent)

		// Labels are different
		node2 = *node
		node2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, CompareServers(node, &node2), Different)

		// Command labels are different
		node2 = *node
		node2.Spec.CmdLabels = map[string]types.CommandLabelV2{"a": {Period: types.Duration(time.Minute), Command: []string{"ls", "-lR"}}}
		require.Equal(t, CompareServers(node, &node2), Different)

		// Address has changed
		node2 = *node
		node2.Spec.Addr = "localhost:3033"
		require.Equal(t, CompareServers(node, &node2), Different)

		// Proxy addr has changed
		node2 = *node
		node2.Spec.PublicAddrs = []string{"localhost:3033"}
		require.Equal(t, CompareServers(node, &node2), Different)

		// Hostname has changed
		node2 = *node
		node2.Spec.Hostname = "luna2"
		require.Equal(t, CompareServers(node, &node2), Different)

		// TeleportVersion has changed
		node2 = *node
		node2.Spec.Version = "5.0.0"
		require.Equal(t, CompareServers(node, &node2), Different)

		// Rotation has changed
		node2 = *node
		node2.Spec.Rotation = types.Rotation{
			State:       types.RotationStateInProgress,
			Phase:       types.RotationPhaseUpdateClients,
			CurrentID:   "1",
			Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
			GracePeriod: types.Duration(3 * time.Hour),
			LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
			Schedule: types.RotationSchedule{
				UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
				Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
			},
		}
		require.Equal(t, CompareServers(node, &node2), Different)
	})

	t.Run("compare DatabaseServices", func(t *testing.T) {
		service := &types.DatabaseServiceV1{
			ResourceHeader: types.ResourceHeader{
				Kind: types.KindDatabaseService,
				Metadata: types.Metadata{
					Name: "dbServiceT01",
				},
			},
			Spec: types.DatabaseServiceSpecV1{
				ResourceMatchers: []*types.DatabaseResourceMatcher{
					{Labels: &types.Labels{"env": []string{"stg"}}},
				},
			},
		}
		service.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))

		// DatabaseService is equal to itself
		require.Equal(t, CompareServers(service, service), Equal)

		// Only timestamps are different
		service2 := *service
		service2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, CompareServers(service, &service2), OnlyTimestampsDifferent)

		// Resource Matcher has changed
		service2 = *service
		service2.Spec.ResourceMatchers = []*types.DatabaseResourceMatcher{
			{Labels: &types.Labels{"env": []string{"stg", "qa"}}},
		}
		require.Equal(t, CompareServers(service, &service2), Different)
	})
}

// TestGuessProxyHostAndVersion checks that the GuessProxyHostAndVersion
// correctly guesses the public address of the proxy (Teleport Cluster).
func TestGuessProxyHostAndVersion(t *testing.T) {
	t.Parallel()

	// No proxies passed in.
	host, version, err := GuessProxyHostAndVersion(nil)
	require.Equal(t, host, "")
	require.Equal(t, version, "")
	require.True(t, trace.IsNotFound(err))

	// No proxies have public address set.
	proxyA := types.ServerV2{}
	proxyA.Spec.Hostname = "test-A"
	proxyA.Spec.Version = "test-A"

	host, version, err = GuessProxyHostAndVersion([]types.Server{&proxyA})
	require.Equal(t, host, fmt.Sprintf("%v:%v", proxyA.Spec.Hostname, defaults.HTTPListenPort))
	require.Equal(t, version, proxyA.Spec.Version)
	require.NoError(t, err)

	// At least one proxy has proxy address set.
	proxyB := types.ServerV2{}
	proxyB.Spec.PublicAddrs = []string{"test-B"}
	proxyB.Spec.Version = "test-B"

	host, version, err = GuessProxyHostAndVersion([]types.Server{&proxyA, &proxyB})
	require.Equal(t, host, proxyB.Spec.PublicAddrs[0])
	require.Equal(t, version, proxyB.Spec.Version)
	require.NoError(t, err)
}

// TestOnlyTimestampsDifferent tests that OnlyTimestampsDifferent is returned
// after checking that whether KubernetesClusters and Apps are different.
func TestOnlyTimestampsDifferent(t *testing.T) {
	t.Parallel()

	now := time.Now()
	later := now.Add(time.Minute)

	tests := []struct {
		desc   string
		a      types.Resource
		b      types.Resource
		expect int
	}{
		{
			desc: "Apps change returns Different",
			a: &types.ServerV2{
				Spec: types.ServerSpecV2{Apps: []*types.App{}},
				Metadata: types.Metadata{
					Expires: &now,
				},
			},
			b: &types.ServerV2{
				Spec: types.ServerSpecV2{Apps: []*types.App{{
					Name: "test-app",
				}}},
				Metadata: types.Metadata{
					Expires: &later,
				},
			},
			expect: Different,
		},
		{
			desc: "No apps change returns OnlyTimestampsDifferent",
			a: &types.ServerV2{
				Spec: types.ServerSpecV2{Apps: []*types.App{}},
				Metadata: types.Metadata{
					Expires: &now,
				},
			},
			b: &types.ServerV2{
				Spec: types.ServerSpecV2{Apps: []*types.App{}},
				Metadata: types.Metadata{
					Expires: &later,
				},
			},
			expect: OnlyTimestampsDifferent,
		},
	}

	for _, tc := range tests {
		got := CompareServers(tc.a, tc.b)
		require.Equal(t, tc.expect, got, tc.desc)
	}
}
