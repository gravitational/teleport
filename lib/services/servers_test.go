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
		require.Equal(t, Equal, CompareServers(node, node))

		// Only timestamps are different
		node2 := *node
		node2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(node, &node2))

		// Labels are different
		node2 = *node
		node2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Different, CompareServers(node, &node2))

		// Command labels are different
		node2 = *node
		node2.Spec.CmdLabels = map[string]types.CommandLabelV2{"a": {Period: types.Duration(time.Minute), Command: []string{"ls", "-lR"}}}
		require.Equal(t, Different, CompareServers(node, &node2))

		// Address has changed
		node2 = *node
		node2.Spec.Addr = "localhost:3033"
		require.Equal(t, Different, CompareServers(node, &node2))

		// Proxy addr has changed
		node2 = *node
		node2.Spec.PublicAddrs = []string{"localhost:3033"}
		require.Equal(t, Different, CompareServers(node, &node2))

		// Hostname has changed
		node2 = *node
		node2.Spec.Hostname = "luna2"
		require.Equal(t, Different, CompareServers(node, &node2))

		// TeleportVersion has changed
		node2 = *node
		node2.Spec.Version = "5.0.0"
		require.Equal(t, Different, CompareServers(node, &node2))

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
		require.Equal(t, Different, CompareServers(node, &node2))
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
		require.Equal(t, Equal, CompareServers(service, service))

		// Only timestamps are different
		service2 := *service
		service2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(service, &service2))

		// Resource Matcher has changed
		service2 = *service
		service2.Spec.ResourceMatchers = []*types.DatabaseResourceMatcher{
			{Labels: &types.Labels{"env": []string{"stg", "qa"}}},
		}
		require.Equal(t, Different, CompareServers(service, &service2))
	})
}

func TestCompareTargetHealth(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Minute)
	tests := []struct {
		name     string
		a        types.TargetHealth
		b        types.TargetHealth
		expected int
	}{
		{
			name:     "equal values",
			a:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg"),
			b:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg"),
			expected: Equal,
		},
		{
			name:     "different timestamps only",
			a:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg"),
			b:        makeHealth("addr", "tcp", "healthy", &later, "reason", "", "msg"),
			expected: OnlyTimestampsDifferent,
		},
		{
			name:     "different address",
			a:        makeHealth("addr1", "tcp", "healthy", &now, "reason", "", "msg"),
			b:        makeHealth("addr2", "tcp", "healthy", &now, "reason", "", "msg"),
			expected: Different,
		},
		{
			name:     "different protocol",
			a:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg"),
			b:        makeHealth("addr", "udp", "healthy", &now, "reason", "", "msg"),
			expected: Different,
		},
		{
			name:     "different status",
			a:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg"),
			b:        makeHealth("addr", "tcp", "unhealthy", &now, "reason", "", "msg"),
			expected: Different,
		},
		{
			name:     "different reason",
			a:        makeHealth("addr", "tcp", "healthy", &now, "reason1", "", "msg"),
			b:        makeHealth("addr", "tcp", "healthy", &now, "reason2", "", "msg"),
			expected: Different,
		},
		{
			name:     "different error",
			a:        makeHealth("addr", "tcp", "healthy", &now, "reason", "err1", "msg"),
			b:        makeHealth("addr", "tcp", "healthy", &now, "reason", "err2", "msg"),
			expected: Different,
		},
		{
			name:     "different message",
			a:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg1"),
			b:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg2"),
			expected: Different,
		},
		{
			name:     "nil timestamp",
			a:        makeHealth("addr", "tcp", "healthy", nil, "reason", "", "msg"),
			b:        makeHealth("addr", "tcp", "healthy", &now, "reason", "", "msg"),
			expected: OnlyTimestampsDifferent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareTargetHealth(tt.a, tt.b)
			require.Equal(t, tt.expected, result)
		})
	}
}

func makeHealth(addr, proto, status string, ts *time.Time, reason, err, msg string) types.TargetHealth {
	return types.TargetHealth{
		Address:             addr,
		Protocol:            proto,
		Status:              status,
		TransitionTimestamp: ts,
		TransitionReason:    reason,
		TransitionError:     err,
		Message:             msg,
	}
}

// TestGuessProxyHostAndVersion checks that the GuessProxyHostAndVersion
// correctly guesses the public address of the proxy (Teleport Cluster).
func TestGuessProxyHostAndVersion(t *testing.T) {
	t.Parallel()

	// No proxies passed in.
	host, version, err := GuessProxyHostAndVersion(nil)
	require.Empty(t, host)
	require.Empty(t, version)
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
