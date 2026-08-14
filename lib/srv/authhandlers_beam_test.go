/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package srv

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/sshca"
)

// TestEvaluateSSHAccessBeamOwnership exercises beam ownership enforcement on
// the exact evaluation path taken by connections to beam nodes: beam nodes
// are agentless OpenSSH nodes, so the proxy's forwarding server authorizes
// them via evaluateSSHAccess. A check anywhere else can be bypassed; this
// test pins the enforcement to the real path.
func TestEvaluateSSHAccessBeamOwnership(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	const clusterName = "localhost"
	const beamID = "beam-123"

	newUserCA := func(t *testing.T, cluster string) (types.CertAuthority, ssh.Signer) {
		t.Helper()
		caPriv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
		require.NoError(t, err)
		ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        types.UserCA,
			ClusterName: cluster,
			ActiveKeys: types.CAKeySet{
				SSH: []*types.SSHKeyPair{{
					PublicKey:      caPriv.MarshalSSHPublicKey(),
					PrivateKey:     caPriv.PrivateKeyPEM(),
					PrivateKeyType: types.PrivateKeyType_RAW,
				}},
			},
		})
		require.NoError(t, err)
		signer, err := ssh.NewSignerFromSigner(caPriv.Signer)
		require.NoError(t, err)
		return ca, signer
	}

	userCA, userCASigner := newUserCA(t, clusterName)
	leafCA, leafCASigner := newUserCA(t, "leaf")

	server := newMockServer(t)
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: clusterName,
		ClusterID:   "cluster_id",
	})
	require.NoError(t, err)
	require.NoError(t, server.auth.SetClusterName(cn))

	accessPoint := &mockCAandAuthPrefGetter{
		AccessPoint: server.auth,
		cas: map[types.CertAuthType][]types.CertAuthority{
			types.UserCA: {userCA, leafCA},
		},
	}
	accessPoint.authPref, err = types.NewAuthPreference(types.AuthPreferenceSpecV2{
		SecondFactor: constants.SecondFactorOff,
	})
	require.NoError(t, err)

	ah, err := NewAuthHandlers(&AuthHandlerConfig{
		Server:                        server,
		Emitter:                       &eventstest.MockRecorderEmitter{},
		AccessPoint:                   accessPoint,
		ValidatedMFAChallengeVerifier: &mockMFAServiceClient{},
	})
	require.NoError(t, err)

	// A deliberately over-broad role: wildcard node access with the beams
	// login. This is exactly the role that made the vulnerability exploitable;
	// the ownership check must hold in spite of it.
	role, err := types.NewRole("access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{types.BeamsLogin, "alice", "bob"},
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)
	_, err = server.auth.CreateRole(ctx, role)
	require.NoError(t, err)
	_, err = server.auth.CreateClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)

	beamNode, err := types.NewServer(beamID, types.KindNode, types.ServerSpecV2{
		Addr:     "1.2.3.4:22",
		Hostname: "beam-host",
		Version:  types.V2,
	})
	require.NoError(t, err)
	beamNode.SetStaticLabels(map[string]string{
		types.BeamIDLabel:    beamID,
		types.BeamOwnerLabel: "alice",
	})

	regularNode, err := types.NewServer("regular", types.KindNode, types.ServerSpecV2{
		Addr:     "5.6.7.8:22",
		Hostname: "regular-host",
		Version:  types.V2,
	})
	require.NoError(t, err)

	issue := func(t *testing.T, signer ssh.Signer, username string) *sshca.Identity {
		t.Helper()
		privateKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
		require.NoError(t, err)
		c, err := testauthority.GenerateUserCert(sshca.UserCertificateRequest{
			CASigner:          signer,
			PublicUserKey:     privateKey.MarshalSSHPublicKey(),
			CertificateFormat: constants.CertificateFormatStandard,
			Identity: sshca.Identity{
				Username:   username,
				Principals: []string{username, types.BeamsLogin},
				Roles:      []string{role.GetName()},
				Traits: wrappers.Traits{
					teleport.TraitInternalPrefix: []string{""},
				},
			},
		})
		require.NoError(t, err)
		cert, err := sshutils.ParseCertificate(c)
		require.NoError(t, err)
		ident, err := sshca.DecodeIdentity(cert)
		require.NoError(t, err)
		return ident
	}

	t.Run("owner allowed on own beam", func(t *testing.T) {
		ident := issue(t, userCASigner, "alice")
		_, err := ah.evaluateSSHAccess(ident, userCA, clusterName, beamNode, types.BeamsLogin)
		require.NoError(t, err)
	})

	t.Run("non-owner denied despite wildcard node access", func(t *testing.T) {
		ident := issue(t, userCASigner, "bob")
		_, err := ah.evaluateSSHAccess(ident, userCA, clusterName, beamNode, types.BeamsLogin)
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, `owned by "alice"`)
	})

	t.Run("non-owner cannot bypass via session join principal", func(t *testing.T) {
		// The moderated-session join path skips node access checks entirely;
		// the ownership check must run before that bypass.
		ident := issue(t, userCASigner, "bob")
		_, err := ah.evaluateSSHAccess(ident, userCA, clusterName, beamNode, teleport.SSHSessionJoinPrincipal)
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, `owned by "alice"`)
	})

	t.Run("cross-cluster identity with owner's username denied", func(t *testing.T) {
		// A leaf-cluster user who happens to share the owner's username must
		// not match: usernames are not unique across clusters.
		ident := issue(t, leafCASigner, "alice")
		_, err := ah.evaluateSSHAccess(ident, leafCA, clusterName, beamNode, types.BeamsLogin)
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, "cross-cluster")
	})

	t.Run("regular node unaffected", func(t *testing.T) {
		ident := issue(t, userCASigner, "bob")
		_, err := ah.evaluateSSHAccess(ident, userCA, clusterName, regularNode, "bob")
		require.NoError(t, err)
	})
}
