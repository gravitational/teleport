/*
Copyright 2016 SPIFFE Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/tlsutils"
)

// ParseKeyStorePEM parses signing key store from PEM encoded key pair
func ParseKeyStorePEM(keyPEM, certPEM string) (*KeyStore, error) {
	_, err := tlsutils.ParseCertificatePEM([]byte(certPEM))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := keys.ParsePrivateKey([]byte(keyPEM))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsaKey, ok := key.Signer.(*rsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("key of type %T is not supported, only RSA keys are supported", key)
	}
	certASN, _ := pem.Decode([]byte(certPEM))
	if certASN == nil {
		return nil, trace.BadParameter("expected PEM-encoded block")
	}
	return &KeyStore{privateKey: rsaKey, cert: certASN.Bytes}, nil
}

// KeyStore is used to sign and decrypt data using X509 digital signatures.
type KeyStore struct {
	privateKey *rsa.PrivateKey
	cert       []byte
}

func (ks *KeyStore) GetKeyPair() (*rsa.PrivateKey, []byte, error) {
	return ks.privateKey, ks.cert, nil
}

// GenerateSelfSignedSigningCert generates self-signed certificate used for digital signatures
func GenerateSelfSignedSigningCert(entity pkix.Name, dnsNames []string, ttl time.Duration) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// to account for clock skew
	notBefore := time.Now().Add(-2 * time.Minute)
	notAfter := notBefore.Add(ttl)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Issuer:                entity,
		Subject:               entity,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return keyPEM, certPEM, nil
}

// ParsePrivateKeyPEM parses PEM-encoded private key.
// Prefer [keys.ParsePrivateKey], this will be deleted after references are removed from teleport.e.
func ParsePrivateKeyPEM(bytes []byte) (crypto.Signer, error) {
	return keys.ParsePrivateKey(bytes)
}

// VerifyCertificateExpiry checks the certificate's expiration status.
func VerifyCertificateExpiry(c *x509.Certificate, clock clockwork.Clock) error {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	now := clock.Now()

	if now.Before(c.NotBefore) {
		return x509.CertificateInvalidError{
			Cert:   c,
			Reason: x509.Expired,
			Detail: fmt.Sprintf("current time %s is before %s", now.UTC().Format(time.RFC3339), c.NotBefore.UTC().Format(time.RFC3339)),
		}
	}
	if now.After(c.NotAfter) {
		return x509.CertificateInvalidError{
			Cert:   c,
			Reason: x509.Expired,
			Detail: fmt.Sprintf("current time %s is after %s", now.UTC().Format(time.RFC3339), c.NotAfter.UTC().Format(time.RFC3339)),
		}
	}
	return nil
}

// VerifyCertificateChain reads in chain of certificates and makes sure the
// chain from leaf to root is valid. This ensures that clients (web browsers
// and CLI) won't have problem validating the chain.
func VerifyCertificateChain(certificateChain []*x509.Certificate) error {
	// chain needs at least one certificate
	if len(certificateChain) == 0 {
		return trace.BadParameter("need at least one certificate in chain")
	}

	// extract leaf of certificate chain. it is safe to index into the chain here
	// because readCertificateChain always returns a valid chain with at least
	// one certificate.
	leaf := certificateChain[0]

	// extract intermediate certificate chain.
	intermediates := x509.NewCertPool()
	if len(certificateChain) > 1 {
		for _, v := range certificateChain[1:] {
			intermediates.AddCert(v)
		}
	}

	// verify certificate chain, roots is nil which will cause us to to use the
	// system roots.
	opts := x509.VerifyOptions{
		Intermediates: intermediates,
	}
	_, err := leaf.Verify(opts)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// IsSelfSigned checks if the certificate is a self-signed certificate. To check
// if a certificate is self-signed, we make sure that only one certificate is in
// the chain and that its Subject and Issuer match.
func IsSelfSigned(certificateChain []*x509.Certificate) bool {
	if len(certificateChain) != 1 {
		return false
	}

	return bytes.Equal(certificateChain[0].RawSubject, certificateChain[0].RawIssuer)
}

// ReadCertificates parses PEM encoded bytes that can contain one or
// multiple certificates and returns a slice of x509.Certificate.
func ReadCertificates(certificateChainBytes []byte) ([]*x509.Certificate, error) {
	var (
		certificateBlock *pem.Block
		certificates     [][]byte
	)
	remainingBytes := bytes.TrimSpace(certificateChainBytes)

	for {
		certificateBlock, remainingBytes = pem.Decode(remainingBytes)
		if certificateBlock == nil {
			return nil, trace.NotFound("no PEM data found")
		}
		if t := certificateBlock.Type; t != pemBlockCertificate {
			return nil, trace.BadParameter("expecting certificate, but found %v", t)
		}
		certificates = append(certificates, certificateBlock.Bytes)

		if len(remainingBytes) == 0 {
			break
		}
	}

	// build concatenated certificates into a buffer
	var buf bytes.Buffer
	for _, cc := range certificates {
		_, err := buf.Write(cc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// parse the buffer and get a slice of x509.Certificates.
	x509Certs, err := x509.ParseCertificates(buf.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return x509Certs, nil
}

// ReadCertificatesFromPath parses PEM encoded certificates from provided path.
func ReadCertificatesFromPath(path string) ([]*x509.Certificate, error) {
	bytes, err := ReadPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := ReadCertificates(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs, nil
}

// NewCertPoolFromPath creates a new x509.CertPool from provided path.
func NewCertPoolFromPath(path string) (*x509.CertPool, error) {
	// x509.CertPool.AppendCertsFromPEM skips parse errors. Using our own
	// implementation here to be more strict.
	cas, err := ReadCertificatesFromPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	for _, ca := range cas {
		pool.AddCert(ca)
	}
	return pool, nil
}

// TLSCertLeaf is a helper function that extracts the parsed leaf *x509.Certificate
// from a tls.Certificate.
// If the leaf certificate is not parsed already, then this function parses it.
func TLSCertLeaf(cert tls.Certificate) (*x509.Certificate, error) {
	if cert.Leaf != nil {
		return cert.Leaf, nil
	}
	if len(cert.Certificate) < 1 {
		return nil, trace.NotFound("invalid certificate length")
	}
	x509cert, err := x509.ParseCertificate(cert.Certificate[0])
	return x509cert, trace.Wrap(err)
}

// InitCertLeaf initializes the Leaf field for each cert in a slice of certs,
// to reduce per-handshake processing.
// Typically, servers should avoid doing this since it will
// consume more memory.
func InitCertLeaf(cert *tls.Certificate) error {
	leaf, err := TLSCertLeaf(*cert)
	if err != nil {
		return trace.Wrap(err)
	}
	cert.Leaf = leaf
	return nil
}

const pemBlockCertificate = "CERTIFICATE"

// CreateCertificateBLOB creates Certificate BLOB
// It has following structure:
//
//	CertificateBlob {
//		PropertyID: u32, little endian,
//		Reserved: u32, little endian, must be set to 0x01 0x00 0x00 0x00
//		Length: u32, little endian
//		Value: certificate data
//	}
//
// Documentation on this structure is a little thin, but one with the structure
// exists in [MS-GPEF]. This doesn't list the `PropertyID` we use below, however
// some references can be found scattered about the internet such as [here].
//
// [MS-GPEF]: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-gpef/e051aba9-c9df-4f82-a42a-c13012c9d381
// [here]: https://github.com/diyinfosec/010-Editor/blob/master/WINDOWS_CERTIFICATE_BLOB.bt
func CreateCertificateBLOB(certData []byte) []byte {
	buf := new(bytes.Buffer)
	buf.Grow(len(certData) + 12)
	// PropertyID for certificate is 32
	binary.Write(buf, binary.LittleEndian, int32(32))
	binary.Write(buf, binary.LittleEndian, int32(1))
	binary.Write(buf, binary.LittleEndian, int32(len(certData)))
	buf.Write(certData)

	return buf.Bytes()
}
