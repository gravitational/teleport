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

package client

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/fixtures"
)

type mockAuthClient struct {
	authclient.ClientI
	server             *auth.Server
	integrationsClient mockIntegrationsClient
}

func (m *mockAuthClient) GetDomainName(ctx context.Context) (string, error) {
	return m.server.GetDomainName()
}

func (m *mockAuthClient) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	return m.server.GetCertAuthorities(ctx, caType, loadKeys)
}

func (m *mockAuthClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return m.server.GetCertAuthority(ctx, id, loadKeys)
}

func (m *mockAuthClient) PerformMFACeremony(ctx context.Context, challengeRequest *proto.CreateAuthenticateChallengeRequest, promptOpts ...mfa.PromptOpt) (*proto.MFAAuthenticateResponse, error) {
	// return MFA not required to gracefully skip the MFA prompt.
	return nil, &mfa.ErrMFANotRequired
}

func (m *mockAuthClient) IntegrationsClient() integrationpb.IntegrationServiceClient {
	return &m.integrationsClient
}

type mockIntegrationsClient struct {
	integrationpb.IntegrationServiceClient
	caKeySet *types.CAKeySet
}

func (m *mockIntegrationsClient) ExportIntegrationCertAuthorities(ctx context.Context, in *integrationpb.ExportIntegrationCertAuthoritiesRequest, opts ...grpc.CallOption) (*integrationpb.ExportIntegrationCertAuthoritiesResponse, error) {
	return &integrationpb.ExportIntegrationCertAuthoritiesResponse{
		CertAuthorities: m.caKeySet,
	}, nil
}

func TestExportAuthorities(t *testing.T) {
	ctx := context.Background()
	const localClusterName = "localcluster"

	testAuth, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: localClusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err, "failed to create auth.NewTestAuthServer")

	validateTLSCertificateDERFunc := func(t *testing.T, s string) {
		cert, err := x509.ParseCertificate([]byte(s))
		require.NoError(t, err, "failed to x509.ParseCertificate")
		require.NotNil(t, cert, "x509.ParseCertificate returned a nil certificate")
		require.Equal(t, localClusterName, cert.Subject.CommonName, "unexpected certificate subject CN")
	}

	validateTLSCertificatePEMFunc := func(t *testing.T, s string) {
		pemBlock, rest := pem.Decode([]byte(s))
		require.NotNil(t, pemBlock, "pem.Decode failed")
		require.Empty(t, rest)

		validateTLSCertificateDERFunc(t, string(pemBlock.Bytes))
	}

	validatePrivateKeyPEMFunc := func(t *testing.T, s string) {
		key, err := keys.ParsePrivateKey([]byte(s))
		require.NoError(t, err)
		require.NotNil(t, key.Signer, "ParsePrivateKey returned a nil key")
	}

	// TestAuthServer uses ECDSA for all CAs except db, db_client, saml_idp, oidc_idp.
	validateRSAPrivateKeyDERFunc := func(t *testing.T, s string) {
		privKey, err := x509.ParsePKCS1PrivateKey([]byte(s))
		require.NoError(t, err, "x509.ParsePKCS1PrivateKey failed")
		require.NotNil(t, privKey, "x509.ParsePKCS1PrivateKey returned a nil key")
	}
	validateECDSAPrivateKeyDERFunc := func(t *testing.T, s string) {
		privKey, err := x509.ParsePKCS8PrivateKey([]byte(s))
		require.NoError(t, err, "x509.ParsePKCS8PrivateKey failed")
		require.NotNil(t, privKey, "x509.ParsePKCS8PrivateKey returned a nil key")
	}

	validateGitHubCAFunc := func(t *testing.T, s string) {
		require.Contains(t, s, fixtures.SSHCAPublicKey)
	}

	for _, exportSecrets := range []bool{false, true} {
		for _, tt := range []struct {
			name            string
			req             ExportAuthoritiesRequest
			errorCheck      require.ErrorAssertionFunc
			assertNoSecrets func(t *testing.T, output string)
			assertSecrets   func(t *testing.T, output string)
		}{
			{
				name: "ssh host and user ca",
				req: ExportAuthoritiesRequest{
					AuthType: "",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output string) {
					require.Contains(t, output, "@cert-authority localcluster,*.localcluster ecdsa-sha2-nistp256")
					require.Contains(t, output, "cert-authority ecdsa-sha2-nistp256")
				},
				assertSecrets: func(t *testing.T, output string) {},
			},
			{
				name: "user",
				req: ExportAuthoritiesRequest{
					AuthType: "user",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output string) {
					require.Contains(t, output, "cert-authority ecdsa-sha2-nistp256")
				},
				assertSecrets: validatePrivateKeyPEMFunc,
			},
			{
				name: "host",
				req: ExportAuthoritiesRequest{
					AuthType: "host",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output string) {
					require.Contains(t, output, "@cert-authority localcluster,*.localcluster ecdsa-sha2-nistp256")
				},
				assertSecrets: validatePrivateKeyPEMFunc,
			},
			{
				name: "tls",
				req: ExportAuthoritiesRequest{
					AuthType: "tls",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificatePEMFunc,
				assertSecrets:   validatePrivateKeyPEMFunc,
			},
			{
				name: "windows",
				req: ExportAuthoritiesRequest{
					AuthType: "windows",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificateDERFunc,
				assertSecrets:   validateECDSAPrivateKeyDERFunc,
			},
			{
				name: "invalid",
				req: ExportAuthoritiesRequest{
					AuthType: "invalid",
				},
				errorCheck: func(tt require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(tt, err, `"invalid" authority type is not supported`)
				},
			},
			{
				name: "fingerprint not found",
				req: ExportAuthoritiesRequest{
					AuthType:                   "user",
					ExportAuthorityFingerprint: "not found fingerprint",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output string) {
					require.Empty(t, output)
				},
				assertSecrets: func(t *testing.T, output string) {
					require.Empty(t, output)
				},
			},
			{
				name: "fingerprint not found",
				req: ExportAuthoritiesRequest{
					AuthType:                   "user",
					ExportAuthorityFingerprint: "fake fingerprint",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output string) {
					require.Empty(t, output)
				},
				assertSecrets: func(t *testing.T, output string) {
					require.Empty(t, output)
				},
			},
			{
				name: "using compat version",
				req: ExportAuthoritiesRequest{
					AuthType:         "user",
					UseCompatVersion: true,
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output string) {
					// compat version (using 1.0) returns cert-authority to be used in the server
					// even when asking for ssh authorized hosts / known hosts
					require.Contains(t, output, "@cert-authority localcluster,*.localcluster ecdsa-sha2-nistp256")
				},
				assertSecrets: validatePrivateKeyPEMFunc,
			},
			{
				name: "db",
				req: ExportAuthoritiesRequest{
					AuthType: "db",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificatePEMFunc,
				assertSecrets:   validatePrivateKeyPEMFunc,
			},
			{
				name: "db-der",
				req: ExportAuthoritiesRequest{
					AuthType: "db-der",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificateDERFunc,
				assertSecrets:   validateRSAPrivateKeyDERFunc,
			},
			{
				name: "db-client",
				req: ExportAuthoritiesRequest{
					AuthType: "db-client",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificatePEMFunc,
				assertSecrets:   validatePrivateKeyPEMFunc,
			},
			{
				name: "db-client-der",
				req: ExportAuthoritiesRequest{
					AuthType: "db-client-der",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificateDERFunc,
				assertSecrets:   validateRSAPrivateKeyDERFunc,
			},
			{
				name: "github missing integration",
				req: ExportAuthoritiesRequest{
					AuthType: "github",
				},
				errorCheck: require.Error,
			},
			{
				name: "github",
				req: ExportAuthoritiesRequest{
					AuthType:    "github",
					Integration: "my-github",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateGitHubCAFunc,
			},
		} {
			t.Run(fmt.Sprintf("%s_exportSecrets_%v", tt.name, exportSecrets), func(t *testing.T) {
				mockedClient := &mockAuthClient{
					server: testAuth.AuthServer,
					integrationsClient: mockIntegrationsClient{
						caKeySet: &types.CAKeySet{
							SSH: []*types.SSHKeyPair{{
								PublicKey: []byte(fixtures.SSHCAPublicKey),
							}},
						},
					},
				}
				var (
					err      error
					exported string
				)
				exportFunc := ExportAuthorities
				checkFunc := tt.assertNoSecrets

				if exportSecrets {
					exportFunc = ExportAuthoritiesSecrets
					checkFunc = tt.assertSecrets
				}

				if checkFunc == nil {
					t.Skip("assert func not provided")
				}

				exported, err = exportFunc(ctx, mockedClient, tt.req)
				tt.errorCheck(t, err)

				if err != nil {
					return
				}

				checkFunc(t, exported)
			})
		}
	}
}
