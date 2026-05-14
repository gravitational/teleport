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

package auth_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	clientpb "github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/cryptosuites"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tlscatest"
)

func Test_getSnowflakeJWTParams(t *testing.T) {
	t.Parallel()
	type args struct {
		accountName string
		userName    string
		publicKey   []byte
	}
	tests := []struct {
		name        string
		args        args
		wantSubject string
		wantIssuer  string
	}{
		{
			name: "only account locator",
			args: args{
				accountName: "abc123",
				userName:    "user1",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "ABC123.USER1",
			wantIssuer:  "ABC123.USER1.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
		{
			name: "GCP",
			args: args{
				accountName: "abc321.us-central1.gcp",
				userName:    "user1",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "ABC321.USER1",
			wantIssuer:  "ABC321.USER1.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
		{
			name: "AWS",
			args: args{
				accountName: "abc321.us-west-2.aws",
				userName:    "user2",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "ABC321.USER2",
			wantIssuer:  "ABC321.USER2.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
		{
			name: "global",
			args: args{
				accountName: "testaccount-user.global",
				userName:    "user2",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "TESTACCOUNT.USER2",
			wantIssuer:  "TESTACCOUNT.USER2.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, issuer := auth.GetSnowflakeJWTParams(context.Background(), tt.args.accountName, tt.args.userName, tt.args.publicKey)

			require.Equal(t, tt.wantSubject, subject)
			require.Equal(t, tt.wantIssuer, issuer)
		})
	}
}

func TestDBCertSigning(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Now())
	const clusterName = "local.me"
	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Clock:       clock,
		ClusterName: clusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	authServer := testAuthServer.AuthServer
	authServer.SetSubCAEnabled(true)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{
		CommonName: "localhost",
	}, privateKey)
	require.NoError(t, err)

	// Set rotation to init phase. New CA will be generated.
	// DB service should use active key to sign certificates.
	// tctl should use new key to sign certificates.
	err = authServer.RotateCertAuthority(t.Context(), types.RotateRequest{
		Type:        types.DatabaseCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	err = authServer.RotateCertAuthority(t.Context(), types.RotateRequest{
		Type:        types.DatabaseClientCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	dbCAs, err := authServer.GetCertAuthorities(t.Context(), types.DatabaseCA, false)
	require.NoError(t, err)
	require.Len(t, dbCAs, 1)
	require.Len(t, dbCAs[0].GetActiveKeys().TLS, 1)
	require.Len(t, dbCAs[0].GetAdditionalTrustedKeys().TLS, 1)
	activeDBCACert := dbCAs[0].GetActiveKeys().TLS[0].Cert
	newDBCACert := dbCAs[0].GetAdditionalTrustedKeys().TLS[0].Cert

	dbClientCAs, err := authServer.GetCertAuthorities(t.Context(), types.DatabaseClientCA, false)
	require.NoError(t, err)
	require.Len(t, dbClientCAs, 1)
	require.Len(t, dbClientCAs[0].GetActiveKeys().TLS, 1)
	require.Len(t, dbClientCAs[0].GetAdditionalTrustedKeys().TLS, 1)
	activeDBClientCACert := dbClientCAs[0].GetActiveKeys().TLS[0].Cert
	newDBClientCACert := dbClientCAs[0].GetAdditionalTrustedKeys().TLS[0].Cert

	// Prepare disabled CA overrides for activeDBClientCACert and
	// newDBClientCACert, but do not create the overrides themselves yet.
	var dbClientCAOverride *subcav1.CertAuthorityOverride
	{
		const chainLength = 2
		externalChain, err := subcaenv.MakeCAChain(chainLength, &subcaenv.CAParams{
			Clock: testAuthServer.Clock(),
		})
		require.NoError(t, err)
		externalRoot := externalChain[chainLength-1]

		subCAEnv := subcaenv.Env{
			Clock:        clock,
			ClusterName:  clusterName,
			ExternalRoot: externalRoot,
		}

		parsedActiveDBClientCert, err := tlsutils.ParseCertificatePEM(activeDBClientCACert)
		require.NoError(t, err)
		parsedNewDBClientCert, err := tlsutils.ParseCertificatePEM(newDBClientCACert)
		require.NoError(t, err)

		// Simulate na old, already rotated CA certificate.
		// It should not influence responses.
		_, oldCertPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
			ClusterName: clusterName,
			NotBefore:   clock.Now().Add(-1 * time.Minute),
			NotAfter:    clock.Now().Add(1 * time.Hour),
		})
		require.NoError(t, err)
		oldCert, err := tlsutils.ParseCertificatePEM(oldCertPEM)
		require.NoError(t, err)

		dbClientCAOverride = &subcav1.CertAuthorityOverride{
			Kind:    types.KindCertAuthorityOverride,
			SubKind: string(types.DatabaseClientCA),
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: clusterName,
			},
			Spec: &subcav1.CertAuthorityOverrideSpec{
				CertificateOverrides: []*subcav1.CertificateOverride{
					subCAEnv.NewDisabledCertificateOverride(t, parsedActiveDBClientCert, nil),
					subCAEnv.NewDisabledCertificateOverride(t, parsedNewDBClientCert, nil),
					subCAEnv.NewDisabledCertificateOverride(t, oldCert, nil),
				},
			},
		}
		// Include external root in the first override's chain.
		dbClientCAOverride.Spec.CertificateOverrides[0].Chain = []string{string(externalRoot.CertPEM)}
		// Active the old override.
		dbClientCAOverride.Spec.CertificateOverrides[2].Disabled = false
	}

	// Map self-signed certificates to their overrides.
	// Certificates absent from the map don't need to change.
	caOverrideCertificateMap := map[string]*subcav1.CertificateOverride{
		string(activeDBClientCACert): dbClientCAOverride.Spec.CertificateOverrides[0],
		string(newDBClientCACert):    dbClientCAOverride.Spec.CertificateOverrides[1],
	}
	getOverrideAwareCertificates := func(
		selfSignedCert []byte,
		includeChain bool,
	) (cert []byte, pubKeyHash string, chain [][]byte) {
		co, ok := caOverrideCertificateMap[string(selfSignedCert)]
		if !ok {
			return selfSignedCert, "", nil // No overrides.
		}

		if includeChain {
			chain = make([][]byte, 0, len(co.Chain)+1)
			chain = append(chain, []byte(co.Certificate))
			for _, pem := range co.Chain {
				chain = append(chain, []byte(pem))
			}
		}

		return []byte(co.Certificate), co.PublicKey, chain
	}

	mustDeleteCAOverrides := func(t *testing.T) {
		t.Helper()
		err := authServer.DeleteCertAuthorityOverride(t.Context(), types.CertAuthorityOverrideID{
			ClusterName: dbClientCAOverride.Metadata.Name,
			CAType:      dbClientCAOverride.SubKind,
		})
		require.NoError(t, err, "DeleteCertAuthorityOverride errored")
	}

	type testCase struct {
		name           string
		requester      clientpb.DatabaseCertRequest_Requester
		extensions     clientpb.DatabaseCertRequest_Extensions
		crlDomain      string
		wantCertSigner []byte
		wantCACerts    [][]byte
		wantKeyUsage   []x509.ExtKeyUsage
		wantCDP        []string

		// Automatically set by CA override tests.
		wantOverrideTrustChain [][]byte
		wantOverrideDetails    *clientpb.CAOverrideCertificateDetails
	}

	tests := []testCase{
		{
			name:           "DB service request is signed by active db client CA and trusts db CAs",
			wantCertSigner: activeDBClientCACert,
			wantCACerts:    [][]byte{activeDBCACert, newDBCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			name:           "tctl request is signed by new db CA and trusts db client CAs",
			requester:      clientpb.DatabaseCertRequest_TCTL,
			wantCertSigner: newDBCACert,
			wantCACerts:    [][]byte{activeDBClientCACert, newDBClientCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		{
			name:           "DB service request for SQL Server databases is signed by active db client and trusts db client CAs",
			extensions:     clientpb.DatabaseCertRequest_WINDOWS_SMARTCARD,
			wantCertSigner: activeDBClientCACert,
			wantCACerts:    [][]byte{activeDBClientCACert, newDBClientCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			name:           "tctl request for SQL Server databases is signed by new db CA and trusts db client CAs",
			requester:      clientpb.DatabaseCertRequest_TCTL,
			extensions:     clientpb.DatabaseCertRequest_WINDOWS_SMARTCARD,
			wantCertSigner: newDBCACert,
			wantCACerts:    [][]byte{activeDBClientCACert, newDBClientCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		{
			name:           "tctl request for SQL Server database with CDPs",
			requester:      clientpb.DatabaseCertRequest_TCTL,
			extensions:     clientpb.DatabaseCertRequest_WINDOWS_SMARTCARD,
			crlDomain:      "example.com",
			wantCertSigner: newDBCACert,
			wantCACerts:    [][]byte{activeDBClientCACert, newDBClientCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			wantCDP:        []string{"ldap:///CN=local.me,CN=TeleportDB,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=example,DC=com?certificateRevocationList?base?objectClass=cRLDistributionPoint"},
		},
	}
	for _, tt := range tests {
		runTest := func(t *testing.T, tt *testCase) {
			certResp, err := authServer.GenerateDatabaseCert(t.Context(), &clientpb.DatabaseCertRequest{
				CSR:                   csr,
				ServerName:            "localhost",
				TTL:                   clientpb.Duration(time.Hour),
				RequesterName:         tt.requester,
				CertificateExtensions: tt.extensions,
				CRLDomain:             tt.crlDomain,
			})
			require.NoError(t, err)

			// Verify CA certs.
			gotCAs := certResp.CACerts
			wantCAs := slices.Clone(tt.wantCACerts)
			slices.SortFunc(gotCAs, comparePEMs)
			slices.SortFunc(wantCAs, comparePEMs)
			if diff := cmp.Diff(wantCAs, gotCAs); diff != "" {
				t.Errorf("resp.CACerts mismatch (-want +got)\n%s", diff)
			}

			// Verify trust chain.
			if diff := cmp.Diff(tt.wantOverrideTrustChain, certResp.TrustChain); diff != "" {
				t.Errorf("resp.TrustChain mismatch (-want +got)\n%s", diff)
			}

			// Verify override details.
			if diff := cmp.Diff(tt.wantOverrideDetails, certResp.CAOverride, protocmp.Transform()); diff != "" {
				t.Errorf("resp.CaOverride mismatch (-want +got)\n%s", diff)
			}

			// verify that the response cert is a DB CA cert.
			mustVerifyCert(t, tt.wantCertSigner, certResp.Cert, tt.wantCDP, tt.wantKeyUsage...)
		}

		t.Run(tt.name, func(t *testing.T) {
			// "Plain" test, no CA overrides.
			t.Run("ok", func(t *testing.T) { runTest(t, &tt) })

			t.Run("disabled_ca_overrides", func(t *testing.T) {
				_, err := authServer.CreateCertAuthorityOverride(t.Context(), dbClientCAOverride)
				require.NoError(t, err, "CreateCertAuthorityOverride errored")
				t.Cleanup(func() { mustDeleteCAOverrides(t) })

				// Disabled overrides have no effect on the outcome.
				runTest(t, &tt)
			})

			t.Run("active_ca_overrides", func(t *testing.T) {
				// Take a copy, then enable all overrides.
				caOverride := proto.Clone(dbClientCAOverride).(*subcav1.CertAuthorityOverride)
				for _, co := range caOverride.Spec.CertificateOverrides {
					co.Disabled = false
				}
				_, err := authServer.CreateCertAuthorityOverride(t.Context(), caOverride)
				require.NoError(t, err, "CreateCertAuthorityOverride errored")
				t.Cleanup(func() { mustDeleteCAOverrides(t) })

				// Adjust wanted cert/CAs to account for overrides.
				wantCertSigner, pubKeyHash, wantChain := getOverrideAwareCertificates(tt.wantCertSigner, true)
				wantCACerts := make([][]byte, len(tt.wantCACerts))
				for i, cert := range tt.wantCACerts {
					wantCACerts[i], _, _ = getOverrideAwareCertificates(cert, false)
				}

				tt := tt // Take a copy.
				tt.wantCertSigner = wantCertSigner
				tt.wantCACerts = wantCACerts
				tt.wantOverrideTrustChain = wantChain
				if pubKeyHash != "" {
					tt.wantOverrideDetails = &clientpb.CAOverrideCertificateDetails{
						PublicKeyHash: pubKeyHash,
					}
				}
				runTest(t, &tt)
			})
		})
	}
}

func comparePEMs(a, b []byte) int {
	return strings.Compare(string(a), string(b))
}

// mustVerifyCert is a helper func that verifies leaf cert with root cert.
func mustVerifyCert(t *testing.T, rootPEM, leafPEM []byte, cdps []string, keyUsages ...x509.ExtKeyUsage) {
	t.Helper()

	leafCert, err := tlsca.ParseCertificatePEM(leafPEM)
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(rootPEM)
	require.True(t, ok)

	require.Equal(t, cdps, leafCert.CRLDistributionPoints)

	opts := x509.VerifyOptions{
		Roots:     certPool,
		KeyUsages: keyUsages,
	}
	// Verify if the generated certificate can be verified with the correct CA.
	_, err = leafCert.Verify(opts)
	require.NoError(t, err)
}

func TestFilterExtensions(t *testing.T) {
	t.Parallel()
	oidA := asn1.ObjectIdentifier{1, 2, 3, 4}
	oidB := asn1.ObjectIdentifier{1, 2, 3, 5}
	extA := pkix.Extension{Id: oidA, Value: []byte("a")}
	extB := pkix.Extension{Id: oidB, Value: []byte("b")}

	tests := []struct {
		name        string
		input       []pkix.Extension
		allowedOIDs []asn1.ObjectIdentifier
		expected    []pkix.Extension
	}{
		{
			name:        "keeps allowed extension",
			input:       []pkix.Extension{extA},
			allowedOIDs: []asn1.ObjectIdentifier{oidA},
			expected:    []pkix.Extension{extA},
		},
		{
			name:        "filters disallowed extension",
			input:       []pkix.Extension{extA},
			allowedOIDs: []asn1.ObjectIdentifier{oidB},
			expected:    []pkix.Extension{},
		},
		{
			name:        "keeps only allowed extension",
			input:       []pkix.Extension{extA, extB},
			allowedOIDs: []asn1.ObjectIdentifier{oidA},
			expected:    []pkix.Extension{extA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := auth.FilterExtensions(context.Background(), slog.Default(), tt.input, tt.allowedOIDs...)
			require.Equal(t, tt.expected, got)
		})
	}
}
