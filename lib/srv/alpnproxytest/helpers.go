// Copyright 2022 Gravitational, Inc
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

package alpnproxytest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

func MustGenSelfSignedCert(t *testing.T) *tlsca.CertAuthority {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "localhost",
	}, []string{"localhost"}, defaults.CATTL)
	require.NoError(t, err)

	ca, err := tlsca.FromKeys(caCert, caKey)
	require.NoError(t, err)
	return ca
}

type signOptions struct {
	identity tlsca.Identity
	clock    clockwork.Clock
}

func WithIdentity(identity tlsca.Identity) SignOptionsFunc {
	return func(o *signOptions) {
		o.identity = identity
	}
}

func WithClock(clock clockwork.Clock) SignOptionsFunc {
	return func(o *signOptions) {
		o.clock = clock
	}
}

type SignOptionsFunc func(o *signOptions)

func MustGenCertSignedWithCA(t *testing.T, ca *tlsca.CertAuthority, opts ...SignOptionsFunc) tls.Certificate {
	options := signOptions{
		identity: tlsca.Identity{Username: "test-user"},
		clock:    clockwork.NewRealClock(),
	}

	for _, opt := range opts {
		opt(&options)
	}

	subj, err := options.identity.Subject()
	require.NoError(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     options.clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  options.clock.Now().UTC().Add(time.Minute),
		DNSNames:  []string{"localhost", "*.localhost"},
	})
	require.NoError(t, err)

	keyRaw := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw})
	cert, err := tls.X509KeyPair(tlsCert, keyPEM)
	require.NoError(t, err)
	return cert
}
