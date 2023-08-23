// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package credentials

import (
	"crypto/rand"
	"crypto/rsa"
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
)

// mockTLSCredentials mocks insecure Client credentials.
// it returns a nil tlsConfig which allows the client to run in insecure mode.
type mockTLSCredentials struct {
	CertificateChain *tls.Certificate
}

func (mc *mockTLSCredentials) Dialer(_ client.Config) (client.ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
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

	caKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	clientKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	validCertBytes, err := x509.CreateCertificate(rand.Reader, validCert, ca, &clientKey.PublicKey, caKey)
	require.NoError(t, err)
	invalidCertBytes, err := x509.CreateCertificate(rand.Reader, expiredCert, ca, &clientKey.PublicKey, caKey)
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
				aggregate, ok := trace.Unwrap(err).(trace.Aggregate)
				require.True(t, ok)
				require.Equal(t, len(aggregate.Errors()), tc.expectNumErrors, "check number of errors reported")
			}
		})
	}

}
