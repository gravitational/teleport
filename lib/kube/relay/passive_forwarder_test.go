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

package relay

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestPassiveForwarder_Selection(t *testing.T) {
	kubeServers := []*apitypes.KubernetesServerV3{
		// not in any group
		{
			Spec: apitypes.KubernetesServerSpecV3{
				RelayGroup: "",
				HostID:     "host-a",
				Cluster: &apitypes.KubernetesClusterV3{
					Metadata: apitypes.Metadata{
						Name: "cluster-a",
					},
				},
			},
		},
		// in the group, unhealthy
		{
			Spec: apitypes.KubernetesServerSpecV3{
				RelayGroup: "thisgroup",
				HostID:     "host-b",
				Cluster: &apitypes.KubernetesClusterV3{
					Metadata: apitypes.Metadata{
						Name: "cluster-b",
					},
				},
			},
			Status: &apitypes.KubernetesServerStatusV3{
				TargetHealth: &apitypes.TargetHealth{
					Status: "unhealthy",
				},
			},
		},
		// not in the correct group
		{
			Spec: apitypes.KubernetesServerSpecV3{
				RelayGroup: "othergroup",
				HostID:     "host-c",
				Cluster: &apitypes.KubernetesClusterV3{
					Metadata: apitypes.Metadata{
						Name: "cluster-c",
					},
				},
			},
		},
		// in the right group, healthy
		{
			Spec: apitypes.KubernetesServerSpecV3{
				RelayGroup: "thisgroup",
				HostID:     "host-b2",
				Cluster: &apitypes.KubernetesClusterV3{
					Metadata: apitypes.Metadata{
						Name: "cluster-b",
					},
				},
			},
			Status: &apitypes.KubernetesServerStatusV3{
				TargetHealth: &apitypes.TargetHealth{
					Status: "healthy",
				},
			},
		},
	}

	var localDialed []string
	fwd, err := NewPassiveForwarder(PassiveForwarderConfig{
		Log:         slog.Default(),
		ClusterName: "clustername",
		GroupName:   "thisgroup",
		GetKubeServersWithFilter: func(ctx context.Context, filter func(readonly.KubeServer) bool) ([]apitypes.KubeServer, error) {
			var out []apitypes.KubeServer
			for _, s := range kubeServers {
				if filter(s) {
					out = append(out, s.Copy())
				}
			}
			return out, nil
		},
		LocalDial: func(ctx context.Context, hostID string, tunnelType apitypes.TunnelType, src, dst net.Addr) (net.Conn, error) {
			localDialed = append(localDialed, hostID)
			return nil, trace.NotFound("no")
		},
		PeerDial: func(ctx context.Context, hostID string, tunnelType apitypes.TunnelType, relayIDs []string, src, dst net.Addr) (net.Conn, error) {
			return nil, trace.ConnectionProblem(nil, "nope")
		},
	})
	require.NoError(t, err)
	defer fwd.Close()

	fwd.forward(SNILabelForKubeCluster("clustername", "cluster-a"), new(bytes.Buffer), (*noopConn)(nil))
	require.Empty(t, localDialed)
	fwd.forward(SNILabelForKubeCluster("clustername", "cluster-c"), new(bytes.Buffer), (*noopConn)(nil))
	require.Empty(t, localDialed)
	// host-b2 is healthy so it will always be tried before the unhealthy host-b
	for range 100 {
		localDialed = []string{}
		fwd.forward(SNILabelForKubeCluster("clustername", "cluster-b"), new(bytes.Buffer), (*noopConn)(nil))
		require.Equal(t, []string{"host-b2.clustername", "host-b.clustername"}, localDialed)
	}
}

type noopConn struct {
	net.Conn
}

func (*noopConn) Close() error         { return nil }
func (*noopConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (*noopConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }
