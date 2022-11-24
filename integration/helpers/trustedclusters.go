// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
)

// WaitForTunnelConnections waits for remote tunnels connections
func WaitForTunnelConnections(t *testing.T, authServer *auth.Server, clusterName string, expectedCount int) {
	t.Helper()
	var conns []types.TunnelConnection
	for i := 0; i < 30; i++ {
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
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v.", trustedCluster, i)
		_, err := authServer.UpsertTrustedCluster(ctx, trustedCluster)
		if err == nil {
			return
		}
		if trace.IsConnectionProblem(err) {
			log.Debugf("Retrying on connection problem: %v.", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if trace.IsAccessDenied(err) {
			log.Debugf("Retrying on access denied: %v.", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		require.FailNow(t, "Terminating on unexpected problem", "%v.", err)
	}
	require.FailNow(t, "Timeout creating trusted cluster")
}

func WaitForClusters(tun reversetunnel.Server, expected int) func() bool {
	return func() bool {
		clusters, err := tun.GetSites()
		if err != nil {
			return false
		}

		// Check the expected number of clusters are connected, and they have all
		// connected with the past 10 seconds.
		if len(clusters) >= expected {
			for _, cluster := range clusters {
				if time.Since(cluster.GetLastConnected()).Seconds() > 10.0 {
					return false
				}
			}
		}

		return true
	}
}

// WaitForNodeCount waits for a certain number of nodes to show up in the remote site.
func WaitForNodeCount(ctx context.Context, t *TeleInstance, clusterName string, count int) error {
	const (
		deadline     = time.Second * 30
		iterWaitTime = time.Second
	)

	err := retryutils.RetryStaticFor(deadline, iterWaitTime, func() error {
		remoteSite, err := t.Tunnel.GetSite(clusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		accessPoint, err := remoteSite.CachingAccessPoint()
		if err != nil {
			return trace.Wrap(err)
		}
		nodes, err := accessPoint.GetNodes(ctx, defaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(nodes) == count {
			return nil
		}
		return trace.BadParameter("found %v nodes, but wanted to find %v nodes", len(nodes), count)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// WaitForActiveTunnelConnections waits for remote cluster to report a minimum number of active connections
func WaitForActiveTunnelConnections(t *testing.T, tunnel reversetunnel.Server, clusterName string, expectedCount int) {
	require.Eventually(t, func() bool {
		cluster, err := tunnel.GetSite(clusterName)
		if err != nil {
			return false
		}
		return cluster.GetTunnelsCount() >= expectedCount
	},
		30*time.Second,
		time.Second,
		"Active tunnel connections did not reach %v in the expected time frame %v", expectedCount, 30*time.Second,
	)
}
