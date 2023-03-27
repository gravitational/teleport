/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

type mockAuthClient struct {
	auth.ClientI
	server *auth.Server
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
		pemBlock, rest := pem.Decode([]byte(s))
		require.NotNil(t, pemBlock, "pem.Decode failed")
		require.Empty(t, rest)

		require.Equal(t, "RSA PRIVATE KEY", pemBlock.Type, "unexpected private key type")

		privKey, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
		require.NoError(t, err, "x509.ParsePKCS1PrivateKey failed")
		require.NotNil(t, privKey, "x509.ParsePKCS1PrivateKey returned a nil certificate")
	}

	validatePrivateKeyDERFunc := func(t *testing.T, s string) {
		privKey, err := x509.ParsePKCS1PrivateKey([]byte(s))
		require.NoError(t, err, "x509.ParsePKCS1PrivateKey failed")
		require.NotNil(t, privKey, "x509.ParsePKCS1PrivateKey returned a nil certificate")
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
					require.Contains(t, output, "@cert-authority localcluster,*.localcluster ssh-rsa")
					require.Contains(t, output, "cert-authority ssh-rsa")
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
					require.Contains(t, output, "cert-authority ssh-rsa")
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
					require.Contains(t, output, "@cert-authority localcluster,*.localcluster ssh-rsa")
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
				assertSecrets:   validatePrivateKeyDERFunc,
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
					require.Contains(t, output, "@cert-authority localcluster,*.localcluster ssh-rsa")
				},
				assertSecrets: validatePrivateKeyPEMFunc,
			},
			{
				name: "db-der",
				req: ExportAuthoritiesRequest{
					AuthType: "db-der",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificateDERFunc,
				assertSecrets:   validatePrivateKeyDERFunc,
			},
		} {
			t.Run(fmt.Sprintf("%s_exportSecrets_%v", tt.name, exportSecrets), func(t *testing.T) {
				mockedClient := &mockAuthClient{
					server: testAuth.AuthServer,
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
