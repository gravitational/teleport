package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

var (
	oidExtKeyUsage = asn1.ObjectIdentifier{2, 5, 29, 37}

	oidExtKeyUsageClientAuth       = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}
	extKeyUsageClientAuthExtension = pkix.Extension{
		Id: oidExtKeyUsage,
		Value: func() []byte {
			val, err := asn1.Marshal([]asn1.ObjectIdentifier{oidExtKeyUsageClientAuth})
			if err != nil {
				panic(err)
			}
			return val
		}(),
	}
)

func TestFoobar(t *testing.T) {
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)
	privateKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	identity := tlsca.Identity{
		Username: "user",
		Groups:   []string{"none"},
	}
	subj, err := identity.Subject()
	require.NoError(t, err)

	certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey:       privateKey.Public(),
		Subject:         subj,
		NotAfter:        time.Now().Add(time.Hour),
		ExtraExtensions: []pkix.Extension{extKeyUsageClientAuthExtension},
	})
	require.NoError(t, err)

	keyRaw := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw})
	cert, err := tls.X509KeyPair(certBytes, keyPEM)
	require.NoError(t, err)
	cc, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	cc = cc
}
