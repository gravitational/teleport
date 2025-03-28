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
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshca"
)

type nativeContext struct {
	suite *test.AuthSuite
}

func setupNativeContext(ctx context.Context, _ *testing.T) *nativeContext {
	var tt nativeContext

	clock := clockwork.NewFakeClockAt(time.Date(2016, 9, 8, 7, 6, 5, 0, time.UTC))

	tt.suite = &test.AuthSuite{
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

	return &tt
}

func TestGenerateKeypairEmptyPass(t *testing.T) {
	t.Parallel()

	tt := setupNativeContext(context.Background(), t)
	tt.suite.GenerateKeypairEmptyPass(t)
}

func TestGenerateHostCert(t *testing.T) {
	t.Parallel()

	tt := setupNativeContext(context.Background(), t)
	tt.suite.GenerateHostCert(t)
}

func TestGenerateUserCert(t *testing.T) {
	t.Parallel()

	tt := setupNativeContext(context.Background(), t)
	tt.suite.GenerateUserCert(t)
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

	tt := setupNativeContext(context.Background(), t)

	caPrivateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	caSigner, err := ssh.NewSignerFromSigner(caPrivateKey)
	require.NoError(t, err)

	hostKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	hostPrivateKey, err := keys.NewSoftwarePrivateKey(hostKey)
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
		hostCertificateBytes, err := tt.suite.A.GenerateHostCert(sshca.HostCertificateRequest{
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

	tt := setupNativeContext(context.Background(), t)

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

		userCertificateBytes, err := tt.suite.A.GenerateUserCert(sshca.UserCertificateRequest{
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
