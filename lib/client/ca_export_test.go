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
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	clientpb "github.com/gravitational/teleport/api/client/proto"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
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

func (m *mockAuthClient) PerformMFACeremony(ctx context.Context, challengeRequest *clientpb.CreateAuthenticateChallengeRequest, promptOpts ...mfa.PromptOpt) (*clientpb.MFAAuthenticateResponse, error) {
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

func TestExportAllAuthorities(t *testing.T) {
	t.Parallel()

	const localClusterName = "localcluster"

	testAuth, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: localClusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err, "failed to create authtest.NewAuthServer")
	t.Cleanup(func() { assert.NoError(t, testAuth.Close()) })

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

	mockedAuthClient := &mockAuthClient{
		server: testAuth.AuthServer,
	}

	t.Run(`"tls-user-der" and "windows" are distinct`, func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		userExports, err := ExportAllAuthorities(ctx, mockedAuthClient, ExportAuthoritiesRequest{
			AuthType: "tls-user-der",
		})
		require.NoError(t, err)
		require.Len(t, userExports, 1)

		windowsExports, err := ExportAllAuthorities(ctx, mockedAuthClient, ExportAuthoritiesRequest{
			AuthType: "windows",
		})
		require.NoError(t, err)
		require.Len(t, windowsExports, 1)

		// "tls-user-der" and "windows" are distinct, which is always true in a
		// fresh cluster.
		// Formats are validated by the test table below.
		assert.NotEqual(t,
			userExports, windowsExports,
			`Exports from "tls-user-der" and "windows" must be distinct`)

		// "windows" export matches the Windows CA.
		windowsCA, err := testAuth.AuthServer.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.WindowsCA,
			DomainName: localClusterName,
		}, false /* loadKeys */)
		require.NoError(t, err)
		cert := windowsCA.GetActiveKeys().TLS[0].Cert
		block, _ := pem.Decode(cert)
		require.NotEmpty(t, block, "pem.Decode() failed")
		certDER := block.Bytes
		assert.Equal(t,
			certDER, windowsExports[0].Data,
			`Exported "windows" certificate doesn't match the WindowsCA certificate`)
	})

	for _, tt := range []struct {
		name            string
		req             ExportAuthoritiesRequest
		errorCheck      require.ErrorAssertionFunc
		assertNoSecrets func(t *testing.T, output string)
		assertSecrets   func(t *testing.T, output string)
		skipSecrets     bool
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
			name: "tls-user-der",
			req: ExportAuthoritiesRequest{
				AuthType: "tls-user-der",
			},
			errorCheck:      require.NoError,
			assertNoSecrets: validateTLSCertificateDERFunc,
			assertSecrets:   validateECDSAPrivateKeyDERFunc,
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
			errorCheck: func(tt require.TestingT, err error, i ...any) {
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
			name: "aws iam roles anywhere",
			req: ExportAuthoritiesRequest{
				AuthType: "awsra",
			},
			errorCheck:      require.NoError,
			assertNoSecrets: validateTLSCertificatePEMFunc,
			assertSecrets:   validatePrivateKeyPEMFunc,
		},
	} {
		runTest := func(
			t *testing.T,
			exportFunc func(context.Context, authclient.ClientI, ExportAuthoritiesRequest) ([]*ExportedAuthority, error),
			assertFunc func(t *testing.T, output string),
		) {
			ctx := t.Context()

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
			if tt.skipSecrets {
				return
			}

			t.Run("ExportAllAuthoritiesSecrets", func(t *testing.T) {
				runTest(t, ExportAllAuthoritiesSecrets, tt.assertSecrets)
			})
		})
	}
}

func TestExportAllAuthorities_additionalKeys(t *testing.T) {
	t.Parallel()

	const clusterName = "zarq"
	testAuth, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: clusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, testAuth.Close()) })

	authServer := testAuth.AuthServer
	ctx := t.Context()

	const caType = types.UserCA
	ca, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: clusterName,
	}, true /* loadKeys */)
	require.NoError(t, err)

	makeNewTLSKey := func(t *testing.T) *types.TLSKeyPair {
		t.Helper()

		const ttl = 1 * time.Hour // Arbitrary. Actual CA TTLs are much larger.
		keyPEM, certPEM, err := tlsca.GenerateSelfSignedCA(
			pkix.Name{
				Organization: []string{clusterName},
				CommonName:   clusterName,
			},
			nil /* dnsNames */, ttl)
		require.NoError(t, err)

		return &types.TLSKeyPair{
			Cert:    certPEM,
			Key:     keyPEM,
			KeyType: types.PrivateKeyType_RAW,
		}
	}

	kp1 := makeNewTLSKey(t)

	// Make sure multiple keys exist in both active and additionalTrusted sets.
	aks := ca.GetActiveKeys()
	aks.TLS = append(aks.TLS,
		makeNewTLSKey(t),
		&types.TLSKeyPair{Cert: kp1.Cert}, // Cert without Key.
		// 3 entries total (existing + new + cert only)
	)
	require.NoError(t, ca.SetActiveKeys(aks))
	tks := ca.GetAdditionalTrustedKeys()
	tks.TLS = append(tks.TLS,
		makeNewTLSKey(t),
		makeNewTLSKey(t),
		// 2 entries total (both new)
	)
	require.NoError(t, ca.SetAdditionalTrustedKeys(tks))

	// Update CA with new keys.
	_, err = authServer.UpdateCertAuthority(ctx, ca)
	require.NoError(t, err)

	var wantCerts, wantKeys []*ExportedAuthority
	for _, keySet := range [][]*types.TLSKeyPair{aks.TLS, tks.TLS} {
		for _, kp := range keySet {
			wantCerts = append(wantCerts, &ExportedAuthority{Data: kp.Cert})
			if len(kp.Key) > 0 {
				wantKeys = append(wantKeys, &ExportedAuthority{Data: kp.Key})
			}
		}
	}
	// Sanity check.
	require.Len(t, wantCerts, 5, "Unexpected number of wanted certs")
	require.Len(t, wantKeys, 4, "Unexpected number of wanted keys")

	authClient := &mockAuthClient{
		server: authServer,
	}

	exportReq := ExportAuthoritiesRequest{
		AuthType: "tls-user", // UserCA TLS certificates in PEM form.
	}

	tests := []struct {
		name       string
		exportFunc func(context.Context, authclient.ClientI, ExportAuthoritiesRequest) ([]*ExportedAuthority, error)
		want       []*ExportedAuthority
	}{
		{
			name:       "certs",
			exportFunc: ExportAllAuthorities,
			want:       wantCerts,
		},
		{
			name:       "secrets",
			exportFunc: ExportAllAuthoritiesSecrets,
			want:       wantKeys,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			got, err := test.exportFunc(ctx, authClient, exportReq)
			require.NoError(t, err)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Export mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

// Tests a scenario similar to
// https://github.com/gravitational/teleport/issues/35444.
func TestExportAllAuthorities_multipleActiveKeys(t *testing.T) {
	t.Parallel()

	softwareKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err, "GeneratePrivateKeyWithAlgorithm errored")
	// Typically the HSM key would be RSA2048, but this is fine for testing
	// purposes.
	hsmKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err, "GeneratePrivateKeyWithAlgorithm errored")

	makeSerialNumber := func() func() *big.Int {
		lastSerialNumber := int64(0)
		return func() *big.Int {
			lastSerialNumber++
			return big.NewInt(lastSerialNumber)
		}
	}()

	const clusterName = "zarq" // fake, doesn't matter for this test.
	makeKeyPairs := func(t *testing.T, key *keys.PrivateKey, keyType types.PrivateKeyType) (sshKP *types.SSHKeyPair, tlsPEM, tlsDER *types.TLSKeyPair) {
		sshPriv, err := key.MarshalSSHPrivateKey()
		require.NoError(t, err, "MarshalSSHPrivateKey errored")
		sshKP = &types.SSHKeyPair{
			PublicKey:      key.MarshalSSHPublicKey(),
			PrivateKey:     sshPriv,
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
		x509CertDER, err := x509.CreateCertificate(rand.Reader, template, template /* parent */, key.Public(), key.Signer)
		require.NoError(t, err, "CreateCertificate errored")
		x509CertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: x509CertDER,
		})
		tlsPEM = &types.TLSKeyPair{
			Cert:    x509CertPEM,
			Key:     key.PrivateKeyPEM(),
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

	softKeySSH, softKeyPEM, softKeyDER := makeKeyPairs(t, softwareKey, types.PrivateKeyType_RAW)
	hsmKeySSH, hsmKeyPEM, hsmKeyDER := makeKeyPairs(t, hsmKey, types.PrivateKeyType_PKCS11)
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
			name: "tls-user-der",
			req: &ExportAuthoritiesRequest{
				AuthType: "tls-user-der",
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
				ctx := t.Context()

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
	return nil, trace.NotFound("not found")
}

func (m *multiCAAuthClient) PerformMFACeremony(
	context.Context,
	*clientpb.CreateAuthenticateChallengeRequest,
	...mfa.PromptOpt,
) (*clientpb.MFAAuthenticateResponse, error) {
	// Skip MFA ceremonies.
	return nil, &mfa.ErrMFANotRequired
}

func TestExportIntegrationAuthorities(t *testing.T) {
	t.Parallel()

	testAuth, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: "localcluster",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, testAuth.Close()) })

	fingerprint, err := sshutils.AuthorizedKeyFingerprint([]byte(fixtures.SSHCAPublicKey))
	require.NoError(t, err)

	mockedAuthClient := &mockAuthClient{
		server: testAuth.AuthServer,
		integrationsClient: mockIntegrationsClient{
			caKeySet: &types.CAKeySet{
				SSH: []*types.SSHKeyPair{{
					PublicKey: []byte(fixtures.SSHCAPublicKey),
				}},
			},
		},
	}

	for _, tc := range []struct {
		name        string
		req         ExportIntegrationAuthoritiesRequest
		checkError  require.ErrorAssertionFunc
		checkOutput func(*testing.T, []*ExportedAuthority)
	}{
		{
			name: "missing integration",
			req: ExportIntegrationAuthoritiesRequest{
				AuthType: "github",
			},
			checkError: require.Error,
		},
		{
			name: "unknown type",
			req: ExportIntegrationAuthoritiesRequest{
				AuthType:    "unknown",
				Integration: "integration",
			},
			checkError: require.Error,
		},
		{
			name: "github",
			req: ExportIntegrationAuthoritiesRequest{
				AuthType:    "github",
				Integration: "integration",
			},
			checkError: require.NoError,
			checkOutput: func(t *testing.T, authorities []*ExportedAuthority) {
				require.Len(t, authorities, 1)
				require.Contains(t, string(authorities[0].Data), fixtures.SSHCAPublicKey)
			},
		},
		{
			name: "matching fingerprint",
			req: ExportIntegrationAuthoritiesRequest{
				AuthType:         "github",
				Integration:      "integration",
				MatchFingerprint: fingerprint,
			},
			checkError: require.NoError,
			checkOutput: func(t *testing.T, authorities []*ExportedAuthority) {
				require.Len(t, authorities, 1)
				require.Contains(t, string(authorities[0].Data), fixtures.SSHCAPublicKey)
			},
		},
		{
			name: "no matching fingerprint",
			req: ExportIntegrationAuthoritiesRequest{
				AuthType:         "github",
				Integration:      "integration",
				MatchFingerprint: "something-does-not-match",
			},
			checkError: require.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			authorities, err := ExportIntegrationAuthorities(ctx, mockedAuthClient, tc.req)
			tc.checkError(t, err)
			if tc.checkOutput != nil {
				tc.checkOutput(t, authorities)
			}
		})
	}
}
