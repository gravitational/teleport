/*
Copyright 2021 Gravitational, Inc.

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

package server

import (
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestRemoteClusterStatus(t *testing.T) {
	a := newTestAuthServer(t)

	rc, err := services.NewRemoteCluster("rc")
	require.NoError(t, err)
	require.NoError(t, a.CreateRemoteCluster(rc))

	wantRC := rc
	// Initially, no tunnels exist and status should be "offline".
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	gotRC, err := a.GetRemoteCluster(rc.GetName())
	gotRC.SetResourceID(0)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC))

	// Create several tunnel connections.
	lastHeartbeat := a.clock.Now().UTC()
	tc1, err := services.NewTunnelConnection("conn-1", services.TunnelConnectionSpecV2{
		ClusterName:   rc.GetName(),
		ProxyName:     "proxy-1",
		LastHeartbeat: lastHeartbeat,
		Type:          services.ProxyTunnel,
	})
	require.NoError(t, err)
	require.NoError(t, a.UpsertTunnelConnection(tc1))

	lastHeartbeat = lastHeartbeat.Add(time.Minute)
	tc2, err := services.NewTunnelConnection("conn-2", services.TunnelConnectionSpecV2{
		ClusterName:   rc.GetName(),
		ProxyName:     "proxy-2",
		LastHeartbeat: lastHeartbeat,
		Type:          services.ProxyTunnel,
	})
	require.NoError(t, err)
	require.NoError(t, a.UpsertTunnelConnection(tc2))

	// With active tunnels, the status is "online" and last_heartbeat is set to
	// the latest tunnel heartbeat.
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	wantRC.SetLastHeartbeat(tc2.GetLastHeartbeat())
	gotRC, err = a.GetRemoteCluster(rc.GetName())
	require.NoError(t, err)
	gotRC.SetResourceID(0)
	require.Empty(t, cmp.Diff(rc, gotRC))

	// Delete the latest connection.
	require.NoError(t, a.DeleteTunnelConnection(tc2.GetClusterName(), tc2.GetName()))

	// The status should remain the same, since tc1 still exists.
	// The last_heartbeat should remain the same, since tc1 has an older
	// heartbeat.
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	gotRC, err = a.GetRemoteCluster(rc.GetName())
	gotRC.SetResourceID(0)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC))

	// Delete the remaining connection
	require.NoError(t, a.DeleteTunnelConnection(tc1.GetClusterName(), tc1.GetName()))

	// The status should switch to "offline".
	// The last_heartbeat should remain the same.
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	gotRC, err = a.GetRemoteCluster(rc.GetName())
	gotRC.SetResourceID(0)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC))
}

func newTestAuthServer(t *testing.T, name ...string) *Server {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { bk.Close() })

	clusterName := "me.localhost"
	if len(name) != 0 {
		clusterName = name[0]
	}
	// Create a cluster with minimal viable config.
	clusterNameRes, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: clusterName,
	})
	require.NoError(t, err)
	authConfig := &InitConfig{
		ClusterName:            clusterNameRes,
		Backend:                bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	a, err := New(authConfig)
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })
	require.NoError(t, a.SetClusterConfig(auth.DefaultClusterConfig()))

	return a
}
