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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
)

func TestRemoteClusterStatus(t *testing.T) {
	ctx := context.Background()
	a := newTestAuthServer(ctx, t)

	rc, err := types.NewRemoteCluster("rc")
	require.NoError(t, err)
	rc, err = a.CreateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	// This scenario deals with only one remote cluster, so it never hits the limit on status updates.
	// TestRefreshRemoteClusters focuses on verifying the update limit logic.
	a.refreshRemoteClusters(ctx)

	wantRC := rc
	// Initially, no tunnels exist and status should be "offline".
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	gotRC, err := a.GetRemoteCluster(ctx, rc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(wantRC, gotRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

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

	a.refreshRemoteClusters(ctx)

	// With active tunnels, the status is "online" and last_heartbeat is set to
	// the latest tunnel heartbeat.
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	wantRC.SetLastHeartbeat(tc2.GetLastHeartbeat())
	gotRC, err = a.GetRemoteCluster(ctx, rc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Delete the latest connection.
	require.NoError(t, a.DeleteTunnelConnection(tc2.GetClusterName(), tc2.GetName()))

	a.refreshRemoteClusters(ctx)

	// The status should remain the same, since tc1 still exists.
	// The last_heartbeat should remain the same, since tc1 has an older
	// heartbeat.
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	gotRC, err = a.GetRemoteCluster(ctx, rc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Delete the remaining connection
	require.NoError(t, a.DeleteTunnelConnection(tc1.GetClusterName(), tc1.GetName()))

	a.refreshRemoteClusters(ctx)

	// The status should switch to "offline".
	// The last_heartbeat should remain the same.
	wantRC.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	gotRC, err = a.GetRemoteCluster(ctx, rc.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestRefreshRemoteClusters(t *testing.T) {
	ctx := context.Background()

	remoteClusterRefreshLimit = 10
	remoteClusterRefreshBuckets = 5

	tests := []struct {
		name               string
		clustersTotal      int
		clustersNeedUpdate int
		expectedUpdates    int
	}{
		{
			name:               "updates all when below the limit",
			clustersTotal:      20,
			clustersNeedUpdate: 7,
			expectedUpdates:    7,
		},
		{
			name:               "updates all when exactly at the limit",
			clustersTotal:      20,
			clustersNeedUpdate: 10,
			expectedUpdates:    10,
		},
		{
			name:               "stops updating after hitting the default limit",
			clustersTotal:      40,
			clustersNeedUpdate: 15,
			expectedUpdates:    10,
		},
		{
			name:               "stops updating after hitting the dynamic limit",
			clustersTotal:      60,
			clustersNeedUpdate: 15,
			expectedUpdates:    13,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.LessOrEqual(t, tt.clustersNeedUpdate, tt.clustersTotal)

			a := newTestAuthServer(ctx, t)

			allClusters := make(map[string]types.RemoteCluster)
			for i := range tt.clustersTotal {
				rc, err := types.NewRemoteCluster(fmt.Sprintf("rc-%03d", i))
				rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
				require.NoError(t, err)
				rc, err = a.CreateRemoteCluster(ctx, rc)
				require.NoError(t, err)
				allClusters[rc.GetName()] = rc

				if i < tt.clustersNeedUpdate {
					lastHeartbeat := a.clock.Now().UTC()
					tc, err := types.NewTunnelConnection(fmt.Sprintf("conn-%03d", i), types.TunnelConnectionSpecV2{
						ClusterName:   rc.GetName(),
						ProxyName:     fmt.Sprintf("proxy-%03d", i),
						LastHeartbeat: lastHeartbeat,
						Type:          types.ProxyTunnel,
					})
					require.NoError(t, err)
					require.NoError(t, a.UpsertTunnelConnection(tc))
				}
			}

			a.refreshRemoteClusters(ctx)

			clusters, err := a.GetRemoteClusters(ctx)
			require.NoError(t, err)

			var updated int
			for _, cluster := range clusters {
				old := allClusters[cluster.GetName()]
				if cmp.Diff(old, cluster, cmpopts.IgnoreFields(types.Metadata{}, "Revision")) != "" {
					updated++
				}
			}

			require.Equal(t, tt.expectedUpdates, updated)
		})
	}
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
		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token: "invalidtoken",
			CAs:   []types.CertAuthority{},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cluster token")
	})

	t.Run("missing CA", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token: validToken,
			CAs:   []types.CertAuthority{},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected exactly one")
	})

	t.Run("more than one CA", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
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
		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token: validToken,
			CAs: []types.CertAuthority{
				suite.NewTestCA(types.UserCA, "rc3"),
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected host certificate authority")
	})

	t.Run("wrong CA name", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
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
		// use the UpsertTrustedCluster in Uncached as we just want the resource
		// in the backend, we don't want to actually connect
		_, err = a.Services.UpsertTrustedCluster(ctx, trustedCluster)
		require.NoError(t, err)

		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token: validToken,
			CAs: []types.CertAuthority{
				suite.NewTestCA(types.HostCA, trustedCluster.GetName()),
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "same name as trusted cluster")
	})

	t.Run("all CAs are returned when v10+", func(t *testing.T) {
		leafClusterCA := types.CertAuthority(suite.NewTestCA(types.HostCA, "leafcluster-1"))
		resp, err := a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token:           validToken,
			CAs:             []types.CertAuthority{leafClusterCA},
			TeleportVersion: teleport.Version,
		})
		require.NoError(t, err)

		require.Len(t, resp.CAs, 4)
		require.ElementsMatch(t,
			[]types.CertAuthType{
				types.HostCA,
				types.UserCA,
				types.DatabaseCA,
				types.OpenSSHCA,
			},
			[]types.CertAuthType{
				resp.CAs[0].GetType(),
				resp.CAs[1].GetType(),
				resp.CAs[2].GetType(),
				resp.CAs[3].GetType(),
			},
		)

		for _, returnedCA := range resp.CAs {
			localCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
				Type:       returnedCA.GetType(),
				DomainName: localClusterName,
			}, false)
			require.NoError(t, err)
			require.True(t, services.CertAuthoritiesEquivalent(localCA, returnedCA))
		}

		rcs, err := a.GetRemoteClusters(ctx)
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

		osshCAs, err := a.GetCertAuthorities(ctx, types.OpenSSHCA, false)
		require.NoError(t, err)
		require.Len(t, osshCAs, 1)
		require.Equal(t, localClusterName, osshCAs[0].GetName())

		// verify that we reject an attempt to re-register the leaf cluster
		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token: validToken,
			CAs:   []types.CertAuthority{leafClusterCA},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "already registered")
	})

	t.Run("Host User and Database CA are returned by default", func(t *testing.T) {
		leafClusterCA := types.CertAuthority(suite.NewTestCA(types.HostCA, "leafcluster-2"))
		resp, err := a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token:           validToken,
			CAs:             []types.CertAuthority{leafClusterCA},
			TeleportVersion: "",
		})
		require.NoError(t, err)

		require.Len(t, resp.CAs, 4)
		require.ElementsMatch(t,
			[]types.CertAuthType{types.HostCA, types.UserCA, types.DatabaseCA, types.OpenSSHCA},
			[]types.CertAuthType{resp.CAs[0].GetType(), resp.CAs[1].GetType(), resp.CAs[2].GetType(), resp.CAs[3].GetType()},
		)
	})

	t.Run("Cloud prohibits adding leaf clusters", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestFeatures: modules.Features{Cloud: true},
		})

		req := &authclient.ValidateTrustedClusterRequest{
			Token: "invalidtoken",
			CAs:   []types.CertAuthority{},
		}

		server := ServerWithRoles{authServer: a}
		_, err := server.ValidateTrustedCluster(ctx, req)
		require.True(t, trace.IsNotImplemented(err), "ValidateTrustedCluster returned an unexpected error, got = %v (%T), want trace.NotImplementedError", err, err)
	})

	t.Run("CA cluster name does not match subject organization", func(t *testing.T) {
		_, err = a.validateTrustedCluster(ctx, &authclient.ValidateTrustedClusterRequest{
			Token: validToken,
			CAs: []types.CertAuthority{
				suite.NewTestCAWithConfig(suite.TestCAConfig{
					Type:                types.HostCA,
					ClusterName:         "remoteCluster",
					SubjectOrganization: "commonName",
				}),
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "the subject organization of a CA certificate does not match the cluster name of the CA")
	})
}

func newTestAuthServer(ctx context.Context, t *testing.T, name ...string) *Server {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

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
		VersionStorage:         NewFakeTeleportVersion(),
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	a, err := NewServer(authConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		bk.Close()
		a.Close()
	})

	require.NoError(t, a.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = a.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = a.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)
	_, err = a.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	return a
}

func TestUpsertTrustedCluster(t *testing.T) {
	ctx := context.Background()
	testAuth, err := NewTestAuthServer(TestAuthServerConfig{
		ClusterName: "localcluster",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	a := testAuth.AuthServer

	const validToken = "validtoken"
	tks, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles: []types.SystemRole{types.RoleTrustedCluster},
			Token: validToken,
		}},
	})
	require.NoError(t, err)

	err = a.SetStaticTokens(tks)
	require.NoError(t, err)

	role, err := types.NewRole("test-role", types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = a.UpsertRole(ctx, role)
	require.NoError(t, err)

	trustedClusterSpec := types.TrustedClusterSpecV2{
		Enabled: true,
		RoleMap: []types.RoleMapping{
			{
				Local:  []string{"test-role"},
				Remote: "someRole",
			},
		},
		ProxyAddress: "localhost",
	}

	trustedCluster, err := types.NewTrustedCluster("trustedcluster", trustedClusterSpec)
	require.NoError(t, err)

	ca := suite.NewTestCA(types.UserCA, "trustedcluster")

	configureCAsForTrustedCluster(trustedCluster, []types.CertAuthority{ca})

	_, err = a.Services.CreateTrustedCluster(ctx, trustedCluster, []types.CertAuthority{ca})
	require.NoError(t, err)

	err = a.createReverseTunnel(ctx, trustedCluster)
	require.NoError(t, err)

	t.Run("Invalid role change", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster("trustedcluster",
			types.TrustedClusterSpecV2{
				Enabled: true,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{"someNewRole"},
						Remote: "someRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		_, err = a.UpsertTrustedClusterV2(ctx, trustedCluster)
		require.ErrorContains(t, err, "someNewRole")
	})
	t.Run("Change role map of existing enabled trusted cluster", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster("trustedcluster",
			types.TrustedClusterSpecV2{
				Enabled: true,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		_, err = a.UpsertTrustedClusterV2(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Disable existing trusted cluster", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster("trustedcluster",
			types.TrustedClusterSpecV2{
				Enabled: false,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		_, err = a.UpsertTrustedClusterV2(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Change role map of existing disabled trusted cluster", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster("trustedcluster",
			types.TrustedClusterSpecV2{
				Enabled: false,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someOtherRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		_, err = a.UpsertTrustedClusterV2(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Enable existing trusted cluster", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster("trustedcluster",
			types.TrustedClusterSpecV2{
				Enabled: true,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someOtherRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		_, err = a.UpsertTrustedClusterV2(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Upsert unmodified trusted cluster", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster("trustedcluster", trustedClusterSpec)
		require.NoError(t, err)
		_, err = a.UpsertTrustedClusterV2(ctx, trustedCluster)
		require.NoError(t, err)
	})
}

func TestUpdateTrustedCluster(t *testing.T) {
	ctx := context.Background()
	testAuth, err := NewTestAuthServer(TestAuthServerConfig{
		ClusterName: "localcluster",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	a := testAuth.AuthServer

	const validToken = "validtoken"
	tks, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles: []types.SystemRole{types.RoleTrustedCluster},
			Token: validToken,
		}},
	})
	require.NoError(t, err)

	err = a.SetStaticTokens(tks)
	require.NoError(t, err)

	role, err := types.NewRole("test-role", types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = a.UpsertRole(ctx, role)
	require.NoError(t, err)

	trustedClusterSpec := types.TrustedClusterSpecV2{
		Enabled: true,
		RoleMap: []types.RoleMapping{
			{
				Local:  []string{"test-role"},
				Remote: "someRole",
			},
		},
		ProxyAddress: "localhost",
	}

	testClusterName := "trustedcluster"
	trustedCluster, err := types.NewTrustedCluster(testClusterName, trustedClusterSpec)
	require.NoError(t, err)

	ca := suite.NewTestCA(types.UserCA, testClusterName)

	configureCAsForTrustedCluster(trustedCluster, []types.CertAuthority{ca})

	_, err = a.Services.CreateTrustedCluster(ctx, trustedCluster, []types.CertAuthority{ca})
	require.NoError(t, err)

	err = a.createReverseTunnel(ctx, trustedCluster)
	require.NoError(t, err)

	t.Run("Invalid role change", func(t *testing.T) {
		existing, err := a.GetTrustedCluster(ctx, testClusterName)
		require.NoError(t, err)
		trustedCluster, err := types.NewTrustedCluster(testClusterName,
			types.TrustedClusterSpecV2{
				Enabled: true,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{"someNewRole"},
						Remote: "someRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		trustedCluster.SetRevision(existing.GetRevision())
		_, err = a.UpdateTrustedCluster(ctx, trustedCluster)
		require.ErrorContains(t, err, "someNewRole")
	})
	t.Run("Change role map of existing enabled trusted cluster", func(t *testing.T) {
		existing, err := a.GetTrustedCluster(ctx, testClusterName)
		require.NoError(t, err)
		trustedCluster, err := types.NewTrustedCluster(testClusterName,
			types.TrustedClusterSpecV2{
				Enabled: true,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		trustedCluster.SetRevision(existing.GetRevision())
		_, err = a.UpdateTrustedCluster(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Disable existing trusted cluster", func(t *testing.T) {
		existing, err := a.GetTrustedCluster(ctx, testClusterName)
		require.NoError(t, err)
		trustedCluster, err := types.NewTrustedCluster(testClusterName,
			types.TrustedClusterSpecV2{
				Enabled: false,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		trustedCluster.SetRevision(existing.GetRevision())
		_, err = a.UpdateTrustedCluster(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Change role map of existing disabled trusted cluster", func(t *testing.T) {
		existing, err := a.GetTrustedCluster(ctx, testClusterName)
		require.NoError(t, err)
		trustedCluster, err := types.NewTrustedCluster(testClusterName,
			types.TrustedClusterSpecV2{
				Enabled: false,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someOtherRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		trustedCluster.SetRevision(existing.GetRevision())
		_, err = a.UpdateTrustedCluster(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Enable existing trusted cluster", func(t *testing.T) {
		existing, err := a.GetTrustedCluster(ctx, testClusterName)
		require.NoError(t, err)
		trustedCluster, err := types.NewTrustedCluster(testClusterName,
			types.TrustedClusterSpecV2{
				Enabled: true,
				RoleMap: []types.RoleMapping{
					{
						Local:  []string{constants.DefaultImplicitRole},
						Remote: "someOtherRole",
					},
				},
				ProxyAddress: "localhost",
			})
		require.NoError(t, err)
		trustedCluster.SetRevision(existing.GetRevision())
		_, err = a.UpdateTrustedCluster(ctx, trustedCluster)
		require.NoError(t, err)
	})
	t.Run("Update unmodified trusted cluster", func(t *testing.T) {
		existing, err := a.GetTrustedCluster(ctx, testClusterName)
		require.NoError(t, err)
		trustedCluster, err := types.NewTrustedCluster(testClusterName, trustedClusterSpec)
		require.NoError(t, err)
		trustedCluster.SetRevision(existing.GetRevision())
		_, err = a.UpdateTrustedCluster(ctx, trustedCluster)
		require.NoError(t, err)
	})

	t.Run("Invalid revision", func(t *testing.T) {
		trustedCluster, err := types.NewTrustedCluster(testClusterName, trustedClusterSpec)
		require.NoError(t, err)
		_, err = a.UpdateTrustedCluster(ctx, trustedCluster)
		require.Error(t, err)
	})
}
