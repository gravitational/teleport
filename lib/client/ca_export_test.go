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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	clientpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

type mockAuthClient struct {
	authclient.ClientI
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

func (m *mockAuthClient) PerformMFACeremony(ctx context.Context, challengeRequest *clientpb.CreateAuthenticateChallengeRequest, promptOpts ...mfa.PromptOpt) (*clientpb.MFAAuthenticateResponse, error) {
	// return MFA not required to gracefully skip the MFA prompt.
	return nil, &mfa.ErrMFANotRequired
}

func TestExportAuthorities(t *testing.T) {
	t.Parallel()

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

	mockedAuthClient := &mockAuthClient{
		server: testAuth.AuthServer,
	}

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
			assertSecrets:   validatePrivateKeyDERFunc,
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
			assertSecrets:   validatePrivateKeyDERFunc,
		},
	} {
		runTest := func(
			t *testing.T,
			exportFunc func(context.Context, authclient.ClientI, ExportAuthoritiesRequest) ([]*ExportedAuthority, error),
			assertFunc func(t *testing.T, output string),
		) {
			authorities, err := exportFunc(ctx, mockedAuthClient, tt.req)
			tt.errorCheck(t, err)
			if err != nil {
				return
			}

			require.Len(t, authorities, 1, "exported authorities mismatch")
			exported := string(authorities[0].Data)
			assertFunc(t, exported)
		}

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("ExportAllAuthorities", func(t *testing.T) {
				runTest(t, ExportAllAuthorities, tt.assertNoSecrets)
			})
			t.Run("ExportAllAuthoritiesSecrets", func(t *testing.T) {
				runTest(t, ExportAllAuthoritiesSecrets, tt.assertSecrets)
			})
		})
	}
}

// Tests a scenario similar to
// https://github.com/gravitational/teleport/issues/35444.
func TestExportAllAuthorities_mutipleActiveKeys(t *testing.T) {
	t.Parallel()

	softwarePrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "GenerateKey errored")
	hsmPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "GenerateKey errored")

	makeSerialNumber := func() func() *big.Int {
		lastSerialNumber := int64(0)
		return func() *big.Int {
			lastSerialNumber++
			return big.NewInt(lastSerialNumber)
		}
	}()

	const clusterName = "zarq" // fake, doesn't matter for this test.
	makeKeyPairs := func(t *testing.T, key crypto.Signer, keyType types.PrivateKeyType) (sshKP *types.SSHKeyPair, tlsPEM, tlsDER *types.TLSKeyPair) {
		sshPub, err := ssh.NewPublicKey(key.Public())
		require.NoError(t, err, "NewPublicKey errored")

		sshPrivBlock, err := ssh.MarshalPrivateKey(key, "" /* comment */)
		require.NoError(t, err, "MarshalPrivateKey errored")
		sshKP = &types.SSHKeyPair{
			PublicKey:      ssh.MarshalAuthorizedKey(sshPub),
			PrivateKey:     pem.EncodeToMemory(sshPrivBlock),
			PrivateKeyType: keyType,
		}

		serialNumber := makeSerialNumber()
		subject := pkix.Name{
			Organization: []string{clusterName},
			SerialNumber: serialNumber.String(),
			CommonName:   clusterName,
		}
		now := time.Now()
		// template mimics an actual user CA certificate.
		template := &x509.Certificate{
			SerialNumber:          serialNumber,
			Issuer:                subject,
			Subject:               subject,
			NotBefore:             now.Add(-1 * time.Second),
			NotAfter:              now.Add(365 * 24 * time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
		}
		x509CertDER, err := x509.CreateCertificate(rand.Reader, template, template /* parent */, key.Public(), key)
		require.NoError(t, err, "CreateCertificate errored")
		x509CertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: x509CertDER,
		})

		keyRaw, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err, "MarshalPKCS8PrivateKey errored")
		keyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: keyRaw,
		})

		tlsPEM = &types.TLSKeyPair{
			Cert:    x509CertPEM,
			Key:     keyPEM,
			KeyType: keyType,
		}

		block, _ := pem.Decode(tlsPEM.Key)
		require.NotNil(t, block, "pem.Decode returned nil block")
		// Note that typically types.TLSKeyPair doesn't hold raw/DER data, this is
		// only used for test convenience.
		tlsDER = &types.TLSKeyPair{
			Cert:    x509CertDER,
			Key:     block.Bytes,
			KeyType: keyType,
		}

		return sshKP, tlsPEM, tlsDER
	}

	softKeySSH, softKeyPEM, softKeyDER := makeKeyPairs(t, softwarePrivateKey, types.PrivateKeyType_RAW)
	hsmKeySSH, hsmKeyPEM, hsmKeyDER := makeKeyPairs(t, hsmPrivateKey, types.PrivateKeyType_PKCS11)
	userCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        "user",
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{
				softKeySSH,
				hsmKeySSH,
			},
			TLS: []*types.TLSKeyPair{
				softKeyPEM,
				hsmKeyPEM,
			},
		},
	})
	require.NoError(t, err, "NewCertAuthority(user) errored")

	authClient := &multiCAAuthClient{
		ClientI:         nil,
		clusterName:     clusterName,
		certAuthorities: []types.CertAuthority{userCA},
	}
	ctx := context.Background()

	tests := []struct {
		name                    string
		req                     *ExportAuthoritiesRequest
		wantPublic, wantPrivate []*ExportedAuthority
	}{
		{
			name: "tls-user",
			req: &ExportAuthoritiesRequest{
				AuthType: "tls-user",
			},
			wantPublic: []*ExportedAuthority{
				{Data: softKeyPEM.Cert},
				{Data: hsmKeyPEM.Cert},
			},
			wantPrivate: []*ExportedAuthority{
				{Data: softKeyPEM.Key},
				{Data: hsmKeyPEM.Key},
			},
		},
		{
			name: "windows",
			req: &ExportAuthoritiesRequest{
				AuthType: "windows",
			},
			wantPublic: []*ExportedAuthority{
				{Data: softKeyDER.Cert},
				{Data: hsmKeyDER.Cert},
			},
			wantPrivate: []*ExportedAuthority{
				{Data: softKeyDER.Key},
				{Data: hsmKeyDER.Key},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			runTest := func(
				t *testing.T,
				exportAllFunc func(context.Context, authclient.ClientI, ExportAuthoritiesRequest) ([]*ExportedAuthority, error),
				want []*ExportedAuthority,
			) {
				got, err := exportAllFunc(ctx, authClient, *test.req)
				require.NoError(t, err, "exportAllFunc errored")
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("Authorities mismatch (-want +got)\n%s", diff)
				}
			}

			t.Run("ExportAllAuthorities", func(t *testing.T) {
				runTest(t, ExportAllAuthorities, test.wantPublic)
			})
			t.Run("ExportAllAuthoritiesSecrets", func(t *testing.T) {
				runTest(t, ExportAllAuthoritiesSecrets, test.wantPrivate)
			})
		})
	}
}

type multiCAAuthClient struct {
	authclient.ClientI

	clusterName     string
	certAuthorities []types.CertAuthority
}

func (m *multiCAAuthClient) GetDomainName(context.Context) (string, error) {
	return m.clusterName, nil
}

func (m *multiCAAuthClient) GetCertAuthority(_ context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	for _, ca := range m.certAuthorities {
		if ca.GetType() == id.Type && ca.GetClusterName() == id.DomainName {
			if !loadKeys {
				ca = ca.WithoutSecrets().(types.CertAuthority)
			}
			return ca, nil
		}
	}
	return nil, nil
}

func (m *multiCAAuthClient) PerformMFACeremony(
	context.Context,
	*clientpb.CreateAuthenticateChallengeRequest,
	...mfa.PromptOpt,
) (*clientpb.MFAAuthenticateResponse, error) {
	// Skip MFA ceremonies.
	return nil, &mfa.ErrMFANotRequired
}
