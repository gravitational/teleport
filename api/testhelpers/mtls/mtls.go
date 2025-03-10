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

package mtls

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/api/utils/keys"
)

type Config struct {
	ServerTLS *tls.Config
	ClientTLS *tls.Config
}

// NewConfig returns an mTLS config.
func NewConfig(t *testing.T) *Config {
	t.Helper()

	caKey, caCert := generateCA(t)
	serverTLS := generateChildTLSConfigFromCA(t, caKey, caCert)
	clientTLS := generateChildTLSConfigFromCA(t, caKey, caCert)
	clientTLS.ServerName = constants.APIDomain

	return &Config{
		ServerTLS: serverTLS,
		ClientTLS: clientTLS,
	}
}

func generateCA(t *testing.T) (*keys.PrivateKey, *x509.Certificate) {
	t.Helper()

	caPub, caPriv, err := cryptopatch.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	caKey, err := keys.NewPrivateKey(caPriv, nil)
	require.NoError(t, err)

	// Create a self signed certificate.

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Minute)
	entity := pkix.Name{
		Organization: []string{"teleport"},
		CommonName:   "localhost",
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Issuer:                entity,
		Subject:               entity,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, template, template, caPub, caKey)
	require.NoError(t, err)

	x509Cert, err := x509.ParseCertificate(caCertDER)
	require.NoError(t, err)

	return caKey, x509Cert
}

func generateChildTLSConfigFromCA(t *testing.T, caKey *keys.PrivateKey, caCert *x509.Certificate) *tls.Config {
	t.Helper()

	pub, priv, err := cryptopatch.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)

	key, err := keys.NewPrivateKey(priv, nil)
	require.NoError(t, err)

	// Create a certificate signed by the CA.

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Minute)
	entity := pkix.Name{
		Organization: []string{"teleport"},
		CommonName:   "localhost",
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               entity,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		DNSNames:              []string{constants.APIDomain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, pub, caKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := key.TLSCertificate(certPEM)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      pool,
	}
}
