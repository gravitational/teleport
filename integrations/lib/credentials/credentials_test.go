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

package credentials

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// mockTLSCredentials mocks insecure Client credentials.
// it returns a nil tlsConfig which allows the client to run in insecure mode.
type mockTLSCredentials struct {
	CertificateChain *tls.Certificate
}

func (mc *mockTLSCredentials) TLSConfig() (*tls.Config, error) {
	if mc.CertificateChain == nil {
		return nil, nil
	}
	return &tls.Config{GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return mc.CertificateChain, nil
	}}, nil
}

func (mc *mockTLSCredentials) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

func (mc *mockTLSCredentials) Expiry() (time.Time, bool) {
	return time.Time{}, true
}

func TestCheckExpiredCredentials(t *testing.T) {
	// Setup the CA and sign the client certs
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName: "teleport-cluster",
		},
		NotBefore: time.Now().Add(-2 * time.Hour),
		NotAfter:  time.Now().Add(2 * time.Hour),
	}
	expiredCert := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName: "access-plugin",
		},
		NotBefore: time.Now().Add(-2 * time.Minute),
		NotAfter:  time.Now().Add(-1 * time.Minute),
	}
	validCert := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName: "access-plugin",
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(1 * time.Hour),
	}

	caKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	clientKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	validCertBytes, err := x509.CreateCertificate(rand.Reader, validCert, ca, clientKey.Public(), caKey)
	require.NoError(t, err)
	invalidCertBytes, err := x509.CreateCertificate(rand.Reader, expiredCert, ca, clientKey.Public(), caKey)
	require.NoError(t, err)

	expiredCred := &mockTLSCredentials{CertificateChain: &tls.Certificate{Certificate: [][]byte{invalidCertBytes}}}
	validCred := &mockTLSCredentials{CertificateChain: &tls.Certificate{Certificate: [][]byte{validCertBytes}}}

	// Doing the real tests
	testCases := []struct {
		name            string
		credentials     []client.Credentials
		expectNumErrors int
		expectIsValid   bool
	}{
		{
			name: "Empty credentials (no certs)",
			credentials: []client.Credentials{
				&mockTLSCredentials{
					CertificateChain: &tls.Certificate{
						Certificate: [][]byte{},
					},
				},
			},
			expectNumErrors: 0,
			expectIsValid:   false,
		},
		{
			name:            "Empty credentials (no TLS config)",
			credentials:     []client.Credentials{&mockTLSCredentials{}},
			expectNumErrors: 0,
			expectIsValid:   false,
		},
		{
			name:            "Single valid credential",
			credentials:     []client.Credentials{validCred},
			expectNumErrors: 0,
			expectIsValid:   true,
		},
		{
			name:            "Single invalid credential",
			credentials:     []client.Credentials{expiredCred},
			expectNumErrors: 1,
			expectIsValid:   false,
		},
		{
			name:            "Valid and invalid credential",
			credentials:     []client.Credentials{validCred, expiredCred},
			expectNumErrors: 1,
			expectIsValid:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid, err := CheckIfExpired(tc.credentials)
			require.Equal(t, tc.expectIsValid, isValid, "check validity")
			if tc.expectNumErrors == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				var aggregate trace.Aggregate
				require.ErrorAs(t, trace.Unwrap(err), &aggregate)
				require.Len(t, aggregate.Errors(), tc.expectNumErrors, "check number of errors reported")
			}
		})
	}

}
