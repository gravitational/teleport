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

package keygen

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshca"
)

type authSuite struct {
	A      sshca.Authority
	Keygen func() ([]byte, []byte, error)
	Clock  clockwork.Clock
}

func checkCertExpiry(t *testing.T, cert []byte, clock clockwork.Clock, before, after time.Duration) {
	t.Helper()

	certificate, err := sshutils.ParseCertificate(cert)
	require.NoError(t, err)

	// Check the valid time is not more than 1 minute before the current time.
	validAfter := time.Unix(int64(certificate.ValidAfter), 0)
	require.Equal(t, validAfter.Unix(), clock.Now().UTC().Add(before).Unix())

	// Check the valid time is not more than 1 hour after the current time.
	validBefore := time.Unix(int64(certificate.ValidBefore), 0)
	require.Equal(t, validBefore.Unix(), clock.Now().UTC().Add(after).Unix())
}

func setupAuthSuite(ctx context.Context) *authSuite {
	clock := clockwork.NewFakeClockAt(time.Now().UTC())
	return &authSuite{
		A: New(ctx, SetClock(clock)),
		Keygen: func() ([]byte, []byte, error) {
			privateKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return privateKey.PrivateKeyPEM(), privateKey.MarshalSSHPublicKey(), nil
		},
		Clock: clock,
	}
}

func TestGenerateKeypairEmptyPass(t *testing.T) {
	t.Parallel()

	suite := setupAuthSuite(context.Background())

	priv, pub, err := suite.Keygen()
	require.NoError(t, err)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)
}

func TestGenerateHostCert(t *testing.T) {
	t.Parallel()

	suite := setupAuthSuite(context.Background())
	priv, pub, err := suite.Keygen()
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	cert, err := suite.A.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "00000000-0000-0000-0000-000000000000",
		NodeName:      "auth.example.com",
		TTL:           time.Hour,
		Identity: sshca.Identity{
			ClusterName: "example.com",
			SystemRole:  types.RoleAdmin,
		},
	})
	require.NoError(t, err)

	checkCertExpiry(t, cert, suite.Clock, -1*time.Minute, time.Hour)
}

func TestGenerateUserCert(t *testing.T) {
	t.Parallel()

	suite := setupAuthSuite(context.Background())
	priv, pub, err := suite.Keygen()
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	cert, err := suite.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Hour,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"centos", "root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)

	checkCertExpiry(t, cert, suite.Clock, -1*time.Minute, time.Hour)

	cert, err = suite.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               -20,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)

	checkCertExpiry(t, cert, suite.Clock, -1*time.Minute, defaults.MinCertDuration)

	_, err = suite.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               0,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)

	checkCertExpiry(t, cert, suite.Clock, -1*time.Minute, defaults.MinCertDuration)

	_, err = suite.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Hour,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)

	inRoles := []string{"role-1", "role-2"}
	impersonator := "alice"
	cert, err = suite.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Hour,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Impersonator:          impersonator,
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
			Roles:                 inRoles,
		},
	})
	require.NoError(t, err)
	parsedCert, err := sshutils.ParseCertificate(cert)
	require.NoError(t, err)

	parsedIdent, err := sshca.DecodeIdentity(parsedCert)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(parsedIdent.Roles, inRoles))

	require.Empty(t, cmp.Diff(parsedIdent.Impersonator, impersonator))

	// Check that MFAVerified and PreviousIdentityExpires are encoded into ssh cert
	clock := clockwork.NewFakeClock()
	cert, err = suite.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Minute,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:                "user",
			Principals:              []string{"root"},
			MFAVerified:             "mfa-device-id",
			PreviousIdentityExpires: clock.Now().Add(time.Hour),
		},
	})
	require.NoError(t, err)
	parsedCert, err = sshutils.ParseCertificate(cert)
	require.NoError(t, err)
	require.Contains(t, parsedCert.Extensions, teleport.CertExtensionMFAVerified)
	require.Equal(t, "mfa-device-id", parsedCert.Extensions[teleport.CertExtensionMFAVerified])
	require.Contains(t, parsedCert.Extensions, teleport.CertExtensionPreviousIdentityExpires)
	prevIDExpires, err := time.Parse(time.RFC3339, parsedCert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
	require.NoError(t, err)
	require.WithinDuration(t, clock.Now().Add(time.Hour), prevIDExpires, time.Second)

	t.Run("device extensions", func(t *testing.T) {
		const devID = "deviceid1"
		const devTag = "devicetag1"
		const devCred = "devicecred1"
		certRaw, err := suite.A.GenerateUserCert(sshca.UserCertificateRequest{
			CASigner:      caSigner, // Required.
			PublicUserKey: pub,      // Required.
			Identity: sshca.Identity{
				Username:           "llama",           // Required.
				Principals:         []string{"llama"}, // Required.
				DeviceID:           devID,
				DeviceAssetTag:     devTag,
				DeviceCredentialID: devCred,
			},
		})
		require.NoError(t, err, "GenerateUserCert failed")

		sshCert, err := sshutils.ParseCertificate(certRaw)
		require.NoError(t, err, "ParseCertificate failed")
		assert.Equal(t, devID, sshCert.Extensions[teleport.CertExtensionDeviceID], "DeviceID mismatch")
		assert.Equal(t, devTag, sshCert.Extensions[teleport.CertExtensionDeviceAssetTag], "AssetTag mismatch")
		assert.Equal(t, devCred, sshCert.Extensions[teleport.CertExtensionDeviceCredentialID], "CredentialID mismatch")
	})

	t.Run("github identity", func(t *testing.T) {
		githubUserID := "1234567"
		githubUsername := "github-user"
		certRaw, err := suite.A.GenerateUserCert(sshca.UserCertificateRequest{
			CASigner:      caSigner, // Required.
			PublicUserKey: pub,      // Required.
			Identity: sshca.Identity{
				Username:       "llama",           // Required.
				Principals:     []string{"llama"}, // Required.
				GitHubUserID:   githubUserID,
				GitHubUsername: githubUsername,
			},
		})
		require.NoError(t, err, "GenerateUserCert failed")

		sshCert, err := sshutils.ParseCertificate(certRaw)
		require.NoError(t, err, "ParseCertificate failed")
		assert.Equal(t, githubUserID, sshCert.Extensions[teleport.CertExtensionGitHubUserID])
		assert.Equal(t, githubUsername, sshCert.Extensions[teleport.CertExtensionGitHubUsername])
	})
}

// TestBuildPrincipals makes sure that the list of principals for a host
// certificate is correctly built.
//   - If the node has role admin, then only the host ID should be listed
//     in the principals field.
//   - If only a host ID is provided, don't include a empty node name
//     this is for backward compatibility.
//   - If both host ID and node name are given, then both should be included
//     on the certificate.
//   - If the host ID and node name are the same, only list one.
func TestBuildPrincipals(t *testing.T) {
	t.Parallel()

	suite := setupAuthSuite(context.Background())

	caPrivateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	caSigner, err := ssh.NewSignerFromSigner(caPrivateKey)
	require.NoError(t, err)

	hostKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	hostPrivateKey, err := keys.NewPrivateKey(hostKey)
	require.NoError(t, err)
	hostPublicKey := hostPrivateKey.MarshalSSHPublicKey()

	tests := []struct {
		desc               string
		inHostID           string
		inNodeName         string
		inClusterName      string
		inRole             types.SystemRole
		outValidPrincipals []string
	}{
		{
			desc:               "admin role",
			inHostID:           "00000000-0000-0000-0000-000000000000",
			inNodeName:         "auth",
			inClusterName:      "example.com",
			inRole:             types.RoleAdmin,
			outValidPrincipals: []string{"00000000-0000-0000-0000-000000000000"},
		},
		{
			desc:          "backward compatibility",
			inHostID:      "11111111-1111-1111-1111-111111111111",
			inNodeName:    "",
			inClusterName: "example.com",
			inRole:        types.RoleNode,
			outValidPrincipals: []string{
				"11111111-1111-1111-1111-111111111111.example.com",
				"11111111-1111-1111-1111-111111111111",
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
			},
		},
		{
			desc:          "dual principals",
			inHostID:      "22222222-2222-2222-2222-222222222222",
			inNodeName:    "proxy",
			inClusterName: "example.com",
			inRole:        types.RoleProxy,
			outValidPrincipals: []string{
				"22222222-2222-2222-2222-222222222222.example.com",
				"22222222-2222-2222-2222-222222222222",
				"proxy.example.com",
				"proxy",
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
			},
		},
		{
			desc:          "deduplicate principals",
			inHostID:      "33333333-3333-3333-3333-333333333333",
			inNodeName:    "33333333-3333-3333-3333-333333333333",
			inClusterName: "example.com",
			inRole:        types.RoleProxy,
			outValidPrincipals: []string{
				"33333333-3333-3333-3333-333333333333.example.com",
				"33333333-3333-3333-3333-333333333333",
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
			},
		},
	}

	// run tests
	for _, tc := range tests {
		t.Logf("Running test case: %q", tc.desc)
		hostCertificateBytes, err := suite.A.GenerateHostCert(sshca.HostCertificateRequest{
			CASigner:      caSigner,
			PublicHostKey: hostPublicKey,
			HostID:        tc.inHostID,
			NodeName:      tc.inNodeName,
			TTL:           time.Hour,
			Identity: sshca.Identity{
				ClusterName: tc.inClusterName,
				SystemRole:  tc.inRole,
			},
		})
		require.NoError(t, err)

		hostCertificate, err := sshutils.ParseCertificate(hostCertificateBytes)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(hostCertificate.ValidPrincipals, tc.outValidPrincipals))
	}
}

// TestUserCertCompatibility makes sure the compatibility flag can be used to
// add to remove roles from certificate extensions.
func TestUserCertCompatibility(t *testing.T) {
	t.Parallel()

	suite := setupAuthSuite(context.Background())

	caKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	caSigner, err := ssh.NewSignerFromSigner(caKey)
	require.NoError(t, err)

	tests := []struct {
		inCompatibility string
		outHasRoles     bool
	}{
		// 0 - standard, has roles
		{
			constants.CertificateFormatStandard,
			true,
		},
		// 1 - oldssh, no roles
		{
			teleport.CertificateFormatOldSSH,
			false,
		},
	}

	// run tests
	for i, tc := range tests {
		comment := fmt.Sprintf("Test %v", i)

		userCertificateBytes, err := suite.A.GenerateUserCert(sshca.UserCertificateRequest{
			CASigner:          caSigner,
			PublicUserKey:     ssh.MarshalAuthorizedKey(caSigner.PublicKey()),
			TTL:               time.Hour,
			CertificateFormat: tc.inCompatibility,
			Identity: sshca.Identity{
				Username:   "user",
				Principals: []string{"centos", "root"},
				Roles:      []string{"foo"},
				CertificateExtensions: []*types.CertExtension{{
					Type:  types.CertExtensionType_SSH,
					Mode:  types.CertExtensionMode_EXTENSION,
					Name:  "login@github.com",
					Value: "hello",
				}},
				PermitAgentForwarding: true,
				PermitPortForwarding:  true,
			},
		})
		require.NoError(t, err, comment)

		userCertificate, err := sshutils.ParseCertificate(userCertificateBytes)
		require.NoError(t, err, comment)

		// Check if we added the roles extension.
		_, ok := userCertificate.Extensions[teleport.CertExtensionTeleportRoles]
		require.Equal(t, ok, tc.outHasRoles, comment)

		// Check if users custom extension was added.
		extVal := userCertificate.Extensions["login@github.com"]
		require.Equal(t, "hello", extVal)
	}
}
