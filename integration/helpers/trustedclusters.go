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

package helpers

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

// WaitForTunnelConnections waits for remote tunnels connections
func WaitForTunnelConnections(t *testing.T, authServer *auth.Server, clusterName string, expectedCount int) {
	t.Helper()
	var conns []types.TunnelConnection
	for range 30 {
		// to speed things up a bit, bypass the auth cache
		conns, err := authServer.Services.GetTunnelConnections(clusterName)
		require.NoError(t, err)
		if len(conns) == expectedCount {
			return
		}
		time.Sleep(1 * time.Second)
	}
	require.Len(t, conns, expectedCount)
}

// TryCreateTrustedCluster performs several attempts to create a trusted cluster,
// retries on connection problems and access denied errors to let caches
// propagate and services to start
//
// Duplicated in tool/tsh/tsh_test.go
func TryCreateTrustedCluster(t *testing.T, authServer *auth.Server, trustedCluster types.TrustedCluster) {
	t.Helper()
	ctx := context.TODO()
	for range 10 {
		_, err := authServer.CreateTrustedCluster(ctx, trustedCluster)
		if err == nil {
			return
		}
		if trace.IsConnectionProblem(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if trace.IsAccessDenied(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		require.FailNow(t, "Terminating on unexpected problem", "%v.", err)
	}
	require.FailNow(t, "Timeout creating trusted cluster")
}

// TryUpdateTrustedCluster performs several attempts to update a trusted cluster,
// retries on connection problems and access denied errors to let caches
// propagate and services to start
func TryUpdateTrustedCluster(t *testing.T, authServer *auth.Server, trustedCluster types.TrustedCluster) {
	t.Helper()
	ctx := context.TODO()
	for range 10 {
		_, err := authServer.UpdateTrustedCluster(ctx, trustedCluster)
		if err == nil {
			return
		}
		if trace.IsConnectionProblem(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if trace.IsAccessDenied(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		require.FailNow(t, "Terminating on unexpected problem", "%v.", err)
	}
	require.FailNow(t, "Timeout creating trusted cluster")
}

// TryUpdateTrustedCluster performs several attempts to upsert a trusted cluster,
// retries on connection problems and access denied errors to let caches
// propagate and services to start
func TryUpsertTrustedCluster(t *testing.T, authServer *auth.Server, trustedCluster types.TrustedCluster, skipNameValidation bool) {
	t.Helper()
	ctx := context.TODO()
	for range 10 {
		var err error
		if skipNameValidation {
			_, err = authServer.UpsertTrustedCluster(ctx, trustedCluster)
		} else {
			_, err = authServer.UpsertTrustedClusterV2(ctx, trustedCluster)
		}
		if err == nil {
			return
		}
		if trace.IsConnectionProblem(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if trace.IsAccessDenied(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		require.FailNow(t, "Terminating on unexpected problem", "%v.", err)
	}
	require.FailNow(t, "Timeout creating trusted cluster")
}

func WaitForClusters(tun reversetunnelclient.Server, expected int) func() bool {
	// GetSites will always return the local site
	expected++

	return func() (ok bool) {
		clusters, err := tun.GetSites()
		if err != nil {
			return false
		}

		if len(clusters) < expected {
			return false
		}

		var live int
		for _, cluster := range clusters {
			if time.Since(cluster.GetLastConnected()) > 10*time.Second {
				continue
			}

			live++
			if live >= expected {
				return true
			}
		}

		return false
	}
}

// WaitForActiveTunnelConnections waits for remote cluster to report a minimum number of active connections
func WaitForActiveTunnelConnections(t *testing.T, tunnel reversetunnelclient.Server, clusterName string, expectedCount int) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		cluster, err := tunnel.GetSite(clusterName)
		if !assert.NoError(t, err, "site not found") {
			return
		}

		assert.GreaterOrEqual(t, cluster.GetTunnelsCount(), expectedCount, "missing tunnels for site")

		assert.Equal(t, teleport.RemoteClusterStatusOnline, cluster.GetStatus(), "cluster not online")

		_, err = cluster.GetClient()
		assert.NoError(t, err, "cluster not yet available")
	},
		90*time.Second,
		time.Second,
		"Active tunnel connections did not reach %v in the expected time frame %v", expectedCount, 30*time.Second,
	)
}

// TrustedClusterSetup is a grouping of configuration options describing the current trusted
// clusters being tested used for passing info about the clusters to be tested to helper functions.
type TrustedClusterSetup struct {
	Aux         *TeleInstance
	Main        *TeleInstance
	Username    string
	ClusterAux  string
	UseJumpHost bool
}

// CheckTrustedClustersCanConnect check the cluster setup described in tcSetup can connect to each other.
func CheckTrustedClustersCanConnect(ctx context.Context, t *testing.T, tcSetup TrustedClusterSetup) {
	aux := tcSetup.Aux
	main := tcSetup.Main
	username := tcSetup.Username
	clusterAux := tcSetup.ClusterAux
	useJumpHost := tcSetup.UseJumpHost

	// ensure cluster that was enabled from disabled still works
	sshPort, _, _ := aux.StartNodeAndProxy(t, "aux-node")

	// Wait for both cluster to see each other via reverse tunnels.
	require.Eventually(t, WaitForClusters(main.Tunnel, 1), 10*time.Second, 1*time.Second,
		"Two clusters do not see each other: tunnels are not working.")

	// Try and connect to a node in the Aux cluster from the Main cluster using
	// direct dialing.
	creds, err := GenerateUserCreds(UserCredsRequest{
		Process:        main.Process,
		Username:       username,
		RouteToCluster: clusterAux,
	})
	require.NoError(t, err)

	tc, err := main.NewClientWithCreds(ClientConfig{
		Login:    username,
		Cluster:  clusterAux,
		Host:     Loopback,
		Port:     sshPort,
		JumpHost: useJumpHost,
	}, *creds)
	require.NoError(t, err)

	// tell the client to trust aux cluster CAs (from secrets). this is the
	// equivalent of 'known hosts' in openssh
	auxCAS, err := aux.Secrets.GetCAs()
	require.NoError(t, err)
	for _, auxCA := range auxCAS {
		err = tc.AddTrustedCA(ctx, auxCA)
		require.NoError(t, err)
	}

	output := &bytes.Buffer{}
	tc.Stdout = output

	cmd := []string{"echo", "hello world"}

	require.Eventually(t, func() bool {
		return tc.SSH(ctx, cmd) == nil
	}, 10*time.Second, 1*time.Second, "Two clusters cannot connect to each other")

	require.Equal(t, "hello world\n", output.String())
}
