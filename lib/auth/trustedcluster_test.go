package auth

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestRemoteClusterStatus(t *testing.T) {
	ctx := context.Background()
	a := newTestAuthServer(ctx, t)

	rc, err := types.NewRemoteCluster("rc")
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
	tc1, err := types.NewTunnelConnection("conn-1", types.TunnelConnectionSpecV2{
		ClusterName:   rc.GetName(),
		ProxyName:     "proxy-1",
		LastHeartbeat: lastHeartbeat,
		Type:          types.ProxyTunnel,
	})
	require.NoError(t, err)
	require.NoError(t, a.UpsertTunnelConnection(tc1))

	lastHeartbeat = lastHeartbeat.Add(time.Minute)
	tc2, err := types.NewTunnelConnection("conn-2", types.TunnelConnectionSpecV2{
		ClusterName:   rc.GetName(),
		ProxyName:     "proxy-2",
		LastHeartbeat: lastHeartbeat,
		Type:          types.ProxyTunnel,
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

func newTestAuthServer(ctx context.Context, t *testing.T, name ...string) *Server {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { bk.Close() })

	clusterName := "me.localhost"
	if len(name) != 0 {
		clusterName = name[0]
	}
	// Create a cluster with minimal viable config.
	clusterNameRes, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: clusterName,
	})
	require.NoError(t, err)
	authConfig := &InitConfig{
		ClusterName:            clusterNameRes,
		Backend:                bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	a, err := NewServer(authConfig)
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })
	require.NoError(t, a.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()))
	require.NoError(t, a.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()))
	require.NoError(t, a.SetClusterConfig(services.DefaultClusterConfig()))

	return a
}
