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
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"log/slog"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

func Test_getSnowflakeJWTParams(t *testing.T) {
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
			subject, issuer := getSnowflakeJWTParams(context.Background(), tt.args.accountName, tt.args.userName, tt.args.publicKey)

			require.Equal(t, tt.wantSubject, subject)
			require.Equal(t, tt.wantIssuer, issuer)
		})
	}
}

func TestDBCertSigning(t *testing.T) {
	t.Parallel()
	authServer, err := NewTestAuthServer(TestAuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: "local.me",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	ctx := context.Background()

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{
		CommonName: "localhost",
	}, privateKey)
	require.NoError(t, err)

	// Set rotation to init phase. New CA will be generated.
	// DB service should use active key to sign certificates.
	// tctl should use new key to sign certificates.
	err = authServer.AuthServer.RotateCertAuthority(ctx, types.RotateRequest{
		Type:        types.DatabaseCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	err = authServer.AuthServer.RotateCertAuthority(ctx, types.RotateRequest{
		Type:        types.DatabaseClientCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	dbCAs, err := authServer.AuthServer.GetCertAuthorities(ctx, types.DatabaseCA, false)
	require.NoError(t, err)
	require.Len(t, dbCAs, 1)
	require.Len(t, dbCAs[0].GetActiveKeys().TLS, 1)
	require.Len(t, dbCAs[0].GetAdditionalTrustedKeys().TLS, 1)
	activeDBCACert := dbCAs[0].GetActiveKeys().TLS[0].Cert
	newDBCACert := dbCAs[0].GetAdditionalTrustedKeys().TLS[0].Cert

	dbClientCAs, err := authServer.AuthServer.GetCertAuthorities(ctx, types.DatabaseClientCA, false)
	require.NoError(t, err)
	require.Len(t, dbClientCAs, 1)
	require.Len(t, dbClientCAs[0].GetActiveKeys().TLS, 1)
	require.Len(t, dbClientCAs[0].GetAdditionalTrustedKeys().TLS, 1)
	activeDBClientCACert := dbClientCAs[0].GetActiveKeys().TLS[0].Cert
	newDBClientCACert := dbClientCAs[0].GetAdditionalTrustedKeys().TLS[0].Cert

	tests := []struct {
		name           string
		requester      proto.DatabaseCertRequest_Requester
		extensions     proto.DatabaseCertRequest_Extensions
		wantCertSigner []byte
		wantCACerts    [][]byte
		wantKeyUsage   []x509.ExtKeyUsage
	}{
		{
			name:           "DB service request is signed by active db client CA and trusts db CAs",
			wantCertSigner: activeDBClientCACert,
			wantCACerts:    [][]byte{activeDBCACert, newDBCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			name:           "tctl request is signed by new db CA and trusts db client CAs",
			requester:      proto.DatabaseCertRequest_TCTL,
			wantCertSigner: newDBCACert,
			wantCACerts:    [][]byte{activeDBClientCACert, newDBClientCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		{
			name:           "DB service request for SQL Server databases is signed by active db client and trusts db client CAs",
			extensions:     proto.DatabaseCertRequest_WINDOWS_SMARTCARD,
			wantCertSigner: activeDBClientCACert,
			wantCACerts:    [][]byte{activeDBClientCACert, newDBClientCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			name:           "tctl request for SQL Server databases is signed by new db CA and trusts db client CAs",
			requester:      proto.DatabaseCertRequest_TCTL,
			extensions:     proto.DatabaseCertRequest_WINDOWS_SMARTCARD,
			wantCertSigner: newDBCACert,
			wantCACerts:    [][]byte{activeDBClientCACert, newDBClientCACert},
			wantKeyUsage:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			certResp, err := authServer.AuthServer.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{
				CSR:                   csr,
				ServerName:            "localhost",
				TTL:                   proto.Duration(time.Hour),
				RequesterName:         tt.requester,
				CertificateExtensions: tt.extensions,
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantCACerts, certResp.CACerts)

			// verify that the response cert is a DB CA cert.
			mustVerifyCert(t, tt.wantCertSigner, certResp.Cert, tt.wantKeyUsage...)
		})
	}
}

// mustVerifyCert is a helper func that verifies leaf cert with root cert.
func mustVerifyCert(t *testing.T, rootPEM, leafPEM []byte, keyUsages ...x509.ExtKeyUsage) {
	t.Helper()
	leafCert, err := tlsca.ParseCertificatePEM(leafPEM)
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(rootPEM)
	require.True(t, ok)
	opts := x509.VerifyOptions{
		Roots:     certPool,
		KeyUsages: keyUsages,
	}
	// Verify if the generated certificate can be verified with the correct CA.
	_, err = leafCert.Verify(opts)
	require.NoError(t, err)
}

func TestFilterExtensions(t *testing.T) {
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
			got := filterExtensions(context.Background(), slog.Default(), tt.input, tt.allowedOIDs...)
			require.Equal(t, tt.expected, got)
		})
	}
}
