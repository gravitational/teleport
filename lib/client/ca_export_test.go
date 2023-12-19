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
	"bytes"
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
	// rotate DB CA so we can test when multiple TLS keypairs are trusted.
	err = testAuth.AuthServer.RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.DatabaseCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	validateTLSCert := func(t *testing.T, certBytes []byte) {
		t.Helper()
		cert, err := x509.ParseCertificate(certBytes)
		require.NoError(t, err, "failed to x509.ParseCertificate")
		require.NotNil(t, cert, "x509.ParseCertificate returned a nil certificate")
		require.Equal(t, localClusterName, cert.Subject.CommonName, "unexpected certificate subject CN")
	}
	validatePrivateKey := func(t *testing.T, keyBytes []byte) {
		t.Helper()
		privKey, err := x509.ParsePKCS1PrivateKey(keyBytes)
		require.NoError(t, err, "x509.ParsePKCS1PrivateKey failed")
		require.NotNil(t, privKey, "x509.ParsePKCS1PrivateKey returned a nil key")
	}

	validateTLSCertificatesDERFunc := func(wantCount int) func(*testing.T, [][]byte) {
		return func(t *testing.T, output [][]byte) {
			t.Helper()
			require.Len(t, output, wantCount, "expected %v der encoded cert(s)", wantCount)
			for _, cert := range output {
				validateTLSCert(t, cert)
			}
		}
	}

	validateTLSCertificatesPEMFunc := func(wantCount int) func(*testing.T, [][]byte) {
		return func(t *testing.T, output [][]byte) {
			t.Helper()
			require.Len(t, output, 1)
			remainingBytes := bytes.TrimSpace(output[0])
			var count int
			for {
				count++
				var pemBlock *pem.Block
				pemBlock, remainingBytes = pem.Decode(remainingBytes)
				require.NotNil(t, pemBlock, "pem.Decode failed for cert %d", count)
				validateTLSCert(t, pemBlock.Bytes)
				remainingBytes = bytes.TrimSpace(remainingBytes)
				if len(remainingBytes) == 0 {
					break
				}
			}
			require.Equal(t, wantCount, count)
		}
	}

	validatePrivateKeysDERFunc := func(wantCount int) func(*testing.T, [][]byte) {
		return func(t *testing.T, output [][]byte) {
			t.Helper()
			require.Len(t, output, wantCount, "expected %v der encoded private key(s)", wantCount)
			for _, key := range output {
				validatePrivateKey(t, key)
			}
		}
	}

	validatePrivateKeysPEMFunc := func(wantCount int) func(*testing.T, [][]byte) {
		return func(t *testing.T, output [][]byte) {
			t.Helper()
			require.Len(t, output, 1)
			remainingBytes := bytes.TrimSpace(output[0])
			var count int
			for {
				count++
				var pemBlock *pem.Block
				pemBlock, remainingBytes = pem.Decode(remainingBytes)
				require.NotNil(t, pemBlock, "pem.Decode failed for key %d. remaining: %s", count, remainingBytes)

				require.Equal(t, "RSA PRIVATE KEY", pemBlock.Type, "unexpected private key type")
				validatePrivateKey(t, pemBlock.Bytes)

				remainingBytes = bytes.TrimSpace(remainingBytes)
				if len(remainingBytes) == 0 {
					break
				}
			}
			require.Equal(t, wantCount, count)
		}
	}

	for _, exportSecrets := range []bool{false, true} {
		for _, tt := range []struct {
			name            string
			req             ExportAuthoritiesRequest
			errorCheck      require.ErrorAssertionFunc
			assertNoSecrets func(t *testing.T, output [][]byte)
			assertSecrets   func(t *testing.T, output [][]byte)
		}{
			{
				name: "ssh host and user ca",
				req: ExportAuthoritiesRequest{
					AuthType: "",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output [][]byte) {
					require.Len(t, output, 1, "ssh host and user ca should be concatenated")
					got := string(output[0])
					require.Contains(t, got, "@cert-authority localcluster,*.localcluster ssh-rsa")
					require.Contains(t, got, "cert-authority ssh-rsa")
				},
				// two keys should be present - one for host ca one for user ca.
				assertSecrets: validatePrivateKeysPEMFunc(2),
			},
			{
				name: "user",
				req: ExportAuthoritiesRequest{
					AuthType: "user",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output [][]byte) {
					require.Len(t, output, 1, "ssh user ca should be concatenated")
					got := string(output[0])
					require.Contains(t, got, "cert-authority ssh-rsa")
				},
				assertSecrets: validatePrivateKeysPEMFunc(1),
			},
			{
				name: "host",
				req: ExportAuthoritiesRequest{
					AuthType: "host",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output [][]byte) {
					require.Len(t, output, 1)
					got := string(output[0])
					require.Contains(t, got, "@cert-authority localcluster,*.localcluster ssh-rsa")
				},
				assertSecrets: validatePrivateKeysPEMFunc(1),
			},
			{
				name: "tls",
				req: ExportAuthoritiesRequest{
					AuthType: "tls",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificatesPEMFunc(1),
				assertSecrets:   validatePrivateKeysPEMFunc(1),
			},
			{
				name: "windows",
				req: ExportAuthoritiesRequest{
					AuthType: "windows",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificatesDERFunc(1),
				assertSecrets:   validatePrivateKeysDERFunc(1),
			},
			{
				name: "invalid",
				req: ExportAuthoritiesRequest{
					AuthType: "invalid",
				},
				errorCheck: func(tt require.TestingT, err error, i ...interface{}) {
					t.Helper()
					require.ErrorContains(tt, err, `"invalid" authority type is not supported`)
				},
			},
			{
				name: "fingerprint not found",
				req: ExportAuthoritiesRequest{
					AuthType:                   "user",
					ExportAuthorityFingerprint: "fake fingerprint",
				},
				errorCheck: require.NoError,
				assertNoSecrets: func(t *testing.T, output [][]byte) {
					t.Helper()
					require.Empty(t, output)
				},
				assertSecrets: func(t *testing.T, output [][]byte) {
					t.Helper()
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
				assertNoSecrets: func(t *testing.T, output [][]byte) {
					// compat version (using 1.0) returns cert-authority to be used in the server
					// even when asking for ssh authorized hosts / known hosts
					require.Len(t, output, 1)
					got := string(output[0])
					require.Contains(t, got, "@cert-authority localcluster,*.localcluster ssh-rsa")
				},
				assertSecrets: validatePrivateKeysPEMFunc(1),
			},
			{
				name: "db-der",
				req: ExportAuthoritiesRequest{
					AuthType: "db-der",
				},
				errorCheck:      require.NoError,
				assertNoSecrets: validateTLSCertificatesDERFunc(2),
				assertSecrets:   validatePrivateKeysDERFunc(2),
			},
			{
				name: "db",
				req: ExportAuthoritiesRequest{
					AuthType: "db",
				},
				errorCheck: require.NoError,
				// during DB CA rotation, there should be two trusted CA keypairs.
				assertNoSecrets: validateTLSCertificatesPEMFunc(2),
				assertSecrets:   validatePrivateKeysPEMFunc(2),
			},
		} {
			t.Run(fmt.Sprintf("%s_exportSecrets_%v", tt.name, exportSecrets), func(t *testing.T) {
				mockedClient := &mockAuthClient{
					server: testAuth.AuthServer,
				}
				var (
					err      error
					exported [][]byte
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
