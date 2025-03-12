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

package auth

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

type mockServerInfoAccessPoint struct {
	clock         *clockwork.FakeClock
	nodes         []types.Server
	nodesErr      error
	serverInfos   map[string]types.ServerInfo
	serverInfoErr error
	updatedLabels map[string]map[string]string
}

func newMockServerInfoAccessPoint() *mockServerInfoAccessPoint {
	return &mockServerInfoAccessPoint{
		clock:         clockwork.NewFakeClock(),
		serverInfos:   make(map[string]types.ServerInfo),
		updatedLabels: make(map[string]map[string]string),
	}
}

func (m *mockServerInfoAccessPoint) GetNodeStream(_ context.Context, _ string) stream.Stream[types.Server] {
	if m.nodesErr != nil {
		return stream.Fail[types.Server](m.nodesErr)
	}
	return stream.Slice(m.nodes)
}

func (m *mockServerInfoAccessPoint) GetServerInfo(_ context.Context, name string) (types.ServerInfo, error) {
	if m.serverInfoErr != nil {
		return nil, m.serverInfoErr
	}
	si, ok := m.serverInfos[name]
	if !ok {
		return nil, trace.NotFound("no server info named %q", name)
	}
	return si, nil
}

func (m *mockServerInfoAccessPoint) UpdateLabels(_ context.Context, req proto.InventoryUpdateLabelsRequest) error {
	m.updatedLabels[req.ServerID] = req.Labels
	return nil
}

func (m *mockServerInfoAccessPoint) GetClock() clockwork.Clock {
	return m.clock
}

func TestReconcileServerInfo(t *testing.T) {
	t.Parallel()

	const serverName = "test-server"

	awsServerInfo, err := types.NewServerInfo(types.Metadata{
		Name: types.ServerInfoNameFromAWS("my-account", "my-instance"),
	}, types.ServerInfoSpecV1{
		NewLabels: map[string]string{"a": "1", "b": "2"},
	})
	require.NoError(t, err)
	regularServerInfo, err := types.NewServerInfo(types.Metadata{
		Name: types.ServerInfoNameFromNodeName(serverName),
	}, types.ServerInfoSpecV1{
		NewLabels: map[string]string{"b": "3", "c": "4"},
	})
	require.NoError(t, err)
	server, err := types.NewServer(serverName, types.KindNode, types.ServerSpecV2{
		CloudMetadata: &types.CloudMetadata{
			AWS: &types.AWSInfo{
				AccountID:  "my-account",
				InstanceID: "my-instance",
			},
		},
	})
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		ap := newMockServerInfoAccessPoint()
		ap.nodes = []types.Server{server}
		ap.serverInfos = map[string]types.ServerInfo{
			awsServerInfo.GetName():     awsServerInfo,
			regularServerInfo.GetName(): regularServerInfo,
		}

		utils.RunTestBackgroundTask(ctx, t, &utils.TestBackgroundTask{
			Name: "ReconcileServerInfos",
			Task: func(ctx context.Context) error {
				return trace.Wrap(ReconcileServerInfos(ctx, ap))
			},
		})

		// Wait until the reconciler finishes processing a batch.
		ap.clock.BlockUntil(1)
		// Check that the right labels were updated.
		require.Equal(t, map[string]string{
			"aws/a":     "1",
			"aws/b":     "2",
			"dynamic/b": "3",
			"dynamic/c": "4",
		}, ap.updatedLabels[serverName])
	})

	t.Run("restart on error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		ap := newMockServerInfoAccessPoint()
		ap.nodes = []types.Server{server}
		ap.nodesErr = trace.Errorf("an error")
		ap.serverInfos = map[string]types.ServerInfo{
			awsServerInfo.GetName():     awsServerInfo,
			regularServerInfo.GetName(): regularServerInfo,
		}

		utils.RunTestBackgroundTask(ctx, t, &utils.TestBackgroundTask{
			Name: "ReconcileServerInfos",
			Task: func(ctx context.Context) error {
				return trace.Wrap(ReconcileServerInfos(ctx, ap))
			},
		})

		// Block until we hit the retryer.
		ap.clock.BlockUntil(1)
		// Return the error at a different place and advance to the next batch.
		ap.nodesErr = nil
		ap.serverInfoErr = trace.Errorf("an error")
		ap.clock.Advance(defaults.MaxWatcherBackoff)
		// Block until we hit the retryer again.
		ap.clock.BlockUntil(1)
		// Clear the error and allow a successful run.
		ap.serverInfoErr = nil
		ap.clock.Advance(defaults.MaxWatcherBackoff)
		// Block until we hit the loop waiter (meaning the server infos were
		// successfully processed).
		ap.clock.BlockUntil(1)
		// Check that the right labels were updated.
		require.Equal(t, map[string]string{
			"aws/a":     "1",
			"aws/b":     "2",
			"dynamic/b": "3",
			"dynamic/c": "4",
		}, ap.updatedLabels[serverName])
	})
}
