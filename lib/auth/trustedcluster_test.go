// Copyright 2021 Gravitational, Inc
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

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
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

func TestValidateTrustedCluster(t *testing.T) {
	const localClusterName = "localcluster"
	const validToken = "validtoken"
	ctx := context.Background()

	testAuth, err := NewTestAuthServer(TestAuthServerConfig{
		ClusterName: localClusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	a := testAuth.AuthServer

	tks, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles: []types.SystemRole{types.RoleTrustedCluster},
			Token: validToken,
		}},
	})
	require.NoError(t, err)

	err = a.SetStaticTokens(tks)
	require.NoError(t, err)

	t.Run("invalid cluster token", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token: "invalidtoken",
			CAs:   []types.CertAuthority{},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cluster token")
	})

	t.Run("missing CA", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token: validToken,
			CAs:   []types.CertAuthority{},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected exactly one")
	})

	t.Run("more than one CA", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token: validToken,
			CAs: []types.CertAuthority{
				suite.NewTestCA(types.HostCA, "rc1"),
				suite.NewTestCA(types.HostCA, "rc2"),
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected exactly one")
	})

	t.Run("wrong CA type", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token: validToken,
			CAs: []types.CertAuthority{
				suite.NewTestCA(types.UserCA, "rc3"),
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected host certificate authority")
	})

	t.Run("wrong CA name", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token: validToken,
			CAs: []types.CertAuthority{
				suite.NewTestCA(types.HostCA, localClusterName),
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "same name as this cluster")
	})

	t.Run("wrong remote CA name", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster("trustedcluster",
			types.TrustedClusterSpecV2{Roles: []string{"nonempty"}})
		require.NoError(t, err)
		// use the UpsertTrustedCluster in Services as we just want the resource in
		// the backend, we don't want to actually connect
		_, err = a.Services.UpsertTrustedCluster(ctx, trustedCluster)
		require.NoError(t, err)

		_, err = a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token: validToken,
			CAs: []types.CertAuthority{
				suite.NewTestCA(types.HostCA, trustedCluster.GetName()),
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "same name as trusted cluster")
	})

	t.Run("all CAs are returned when v10+", func(t *testing.T) {
		leafClusterCA := types.CertAuthority(suite.NewTestCA(types.HostCA, "leafcluster"))
		resp, err := a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token:           validToken,
			CAs:             []types.CertAuthority{leafClusterCA},
			TeleportVersion: teleport.Version,
		})
		require.NoError(t, err)

		require.Len(t, resp.CAs, 3)
		require.ElementsMatch(t,
			[]types.CertAuthType{types.HostCA, types.UserCA, types.DatabaseCA},
			[]types.CertAuthType{resp.CAs[0].GetType(), resp.CAs[1].GetType(), resp.CAs[2].GetType()},
		)

		for _, returnedCA := range resp.CAs {
			localCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
				Type:       returnedCA.GetType(),
				DomainName: localClusterName,
			}, false)
			require.NoError(t, err)
			require.True(t, services.CertAuthoritiesEquivalent(localCA, returnedCA))
		}

		rcs, err := a.GetRemoteClusters()
		require.NoError(t, err)
		require.Len(t, rcs, 1)
		require.Equal(t, leafClusterCA.GetName(), rcs[0].GetName())

		hostCAs, err := a.GetCertAuthorities(ctx, types.HostCA, false)
		require.NoError(t, err)
		require.Len(t, hostCAs, 2)
		require.ElementsMatch(t,
			[]string{localClusterName, leafClusterCA.GetName()},
			[]string{hostCAs[0].GetName(), hostCAs[1].GetName()},
		)
		require.Empty(t, hostCAs[0].GetRoles())
		require.Empty(t, hostCAs[0].GetRoleMap())
		require.Empty(t, hostCAs[1].GetRoles())
		require.Empty(t, hostCAs[1].GetRoleMap())

		userCAs, err := a.GetCertAuthorities(ctx, types.UserCA, false)
		require.NoError(t, err)
		require.Len(t, userCAs, 1)
		require.Equal(t, localClusterName, userCAs[0].GetName())

		dbCAs, err := a.GetCertAuthorities(ctx, types.DatabaseCA, false)
		require.NoError(t, err)
		require.Len(t, dbCAs, 1)
		require.Equal(t, localClusterName, dbCAs[0].GetName())
	})

	t.Run("only Host and User CA are returned for v9", func(t *testing.T) {
		leafClusterCA := types.CertAuthority(suite.NewTestCA(types.HostCA, "leafcluster"))
		resp, err := a.validateTrustedCluster(ctx, &ValidateTrustedClusterRequest{
			Token:           validToken,
			CAs:             []types.CertAuthority{leafClusterCA},
			TeleportVersion: "",
		})
		require.NoError(t, err)

		require.Len(t, resp.CAs, 2)
		require.ElementsMatch(t,
			[]types.CertAuthType{types.HostCA, types.UserCA},
			[]types.CertAuthType{resp.CAs[0].GetType(), resp.CAs[1].GetType()},
		)
	})
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
	clusterNameRes, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
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
	require.NoError(t, a.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	require.NoError(t, a.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()))
	require.NoError(t, a.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()))
	require.NoError(t, a.SetAuthPreference(ctx, types.DefaultAuthPreference()))
	return a
}

func TestRemoteDBCAMigration(t *testing.T) {
	const (
		localClusterName  = "localcluster"
		remoteClusterName = "trustedcluster"
	)
	ctx := context.Background()

	testAuth, err := NewTestAuthServer(TestAuthServerConfig{
		ClusterName: localClusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	a := testAuth.AuthServer

	trustedCluster, err := types.NewTrustedCluster(remoteClusterName,
		types.TrustedClusterSpecV2{Roles: []string{"nonempty"}})
	require.NoError(t, err)
	// use the UpsertTrustedCluster in Services as we just want the resource in
	// the backend, we don't want to actually connect
	_, err = a.Services.UpsertTrustedCluster(ctx, trustedCluster)
	require.NoError(t, err)

	// Generate remote HostCA and remove private key as remote CA should have only public cert.
	remoteHostCA := suite.NewTestCA(types.HostCA, remoteClusterName)
	types.RemoveCASecrets(remoteHostCA)

	err = a.UpsertCertAuthority(remoteHostCA)
	require.NoError(t, err)

	// Run the migration
	err = migrateDBAuthority(ctx, a)
	require.NoError(t, err)

	dbCAs, err := a.GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: remoteClusterName,
	}, true)
	require.NoError(t, err)
	// Certificate should be copied.
	require.Equal(t, remoteHostCA.Spec.ActiveKeys.TLS[0].Cert, dbCAs.GetActiveKeys().TLS[0].Cert)
	// Private key should be empty.
	require.Nil(t, dbCAs.GetActiveKeys().TLS[0].Key)
}
