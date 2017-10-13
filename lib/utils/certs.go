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
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ParseSigningKeyStore parses signing key store from PEM encoded key pair
func ParseSigningKeyStorePEM(keyPEM, certPEM string) (*SigningKeyStore, error) {
	_, err := ParseCertificatePEM([]byte(certPEM))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := ParsePrivateKeyPEM([]byte(keyPEM))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("key of type %T is not supported, only RSA keys are supported for signatures")
	}
	certASN, _ := pem.Decode([]byte(certPEM))
	if certASN == nil {
		return nil, trace.BadParameter("expected PEM-encoded block")
	}
	return &SigningKeyStore{privateKey: rsaKey, cert: certASN.Bytes}, nil
}

// SigningKeyStore is used to sign using X509 digital signatures
type SigningKeyStore struct {
	privateKey *rsa.PrivateKey
	cert       []byte
}

func (ks *SigningKeyStore) GetKeyPair() (*rsa.PrivateKey, []byte, error) {
	return ks.privateKey, ks.cert, nil
}

// GenerateSelfSignedSigningCert generates self-signed certificate used for digital signatures
func GenerateSelfSignedSigningCert(entity pkix.Name, dnsNames []string, ttl time.Duration) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
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

// ParseCertificateRequestPEM parses PEM-encoded certificate signing request
func ParseCertificateRequestPEM(bytes []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("expected PEM-encoded block")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return csr, nil
}

// ParseCertificatePEM parses PEM-encoded certificate
func ParseCertificatePEM(bytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("expected PEM-encoded block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return cert, nil
}

// ParsePrivateKeyPEM parses PEM-encoded private key
func ParsePrivateKeyPEM(bytes []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("expected PEM-encoded block")
	}
	return ParsePrivateKeyDER(block.Bytes)
}

// ParsePrivateKeyDER parses unencrypted DER-encoded private key
func ParsePrivateKeyDER(der []byte) (crypto.Signer, error) {
	generalKey, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		generalKey, err = x509.ParsePKCS1PrivateKey(der)
		if err != nil {
			generalKey, err = x509.ParseECPrivateKey(der)
			if err != nil {
				log.Errorf("failed to parse key: %v", err)
				return nil, trace.BadParameter("failed parsing private key")
			}
		}
	}

	switch generalKey.(type) {
	case *rsa.PrivateKey:
		return generalKey.(*rsa.PrivateKey), nil
	case *ecdsa.PrivateKey:
		return generalKey.(*ecdsa.PrivateKey), nil
	}

	return nil, trace.BadParameter("unsupported private key type")
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
		for _, v := range certificateChain[1:len(certificateChain)] {
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

// IsSelfSigned checks if the certificate is a self-signed certificate. To
// check if a certificate is self signed, we make sure that only one
// certificate is in the chain and that the SubjectKeyId and AuthorityKeyId
// match.
//
// From RFC5280: https://tools.ietf.org/html/rfc5280#section-4.2.1.1
//
//   The signature on a self-signed certificate is generated with the private
//   key associated with the certificate's subject public key.  (This
//   proves that the issuer possesses both the public and private keys.)
//   In this case, the subject and authority key identifiers would be
//   identical, but only the subject key identifier is needed for
//   certification path building.
//
func IsSelfSigned(certificateChain []*x509.Certificate) bool {
	if len(certificateChain) != 1 {
		return false
	}

	if bytes.Compare(certificateChain[0].SubjectKeyId, certificateChain[0].AuthorityKeyId) != 0 {
		return false
	}

	return true
}

// ReadCertificateChain parses PEM encoded bytes that can contain one or
// multiple certificates and returns a slice of x509.Certificate.
func ReadCertificateChain(certificateChainBytes []byte) ([]*x509.Certificate, error) {
	// build the certificate chain next
	var certificateBlock *pem.Block
	var remainingBytes []byte = bytes.TrimSpace(certificateChainBytes)
	var certificateChain [][]byte

	for {
		certificateBlock, remainingBytes = pem.Decode(remainingBytes)
		if certificateBlock == nil || certificateBlock.Type != pemBlockCertificate {
			return nil, trace.NotFound("no PEM data found")
		}
		certificateChain = append(certificateChain, certificateBlock.Bytes)

		if len(remainingBytes) == 0 {
			break
		}
	}

	// build a concatenated certificate chain
	var buf bytes.Buffer
	for _, cc := range certificateChain {
		_, err := buf.Write(cc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// parse the chain and get a slice of x509.Certificates.
	x509Chain, err := x509.ParseCertificates(buf.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return x509Chain, nil
}

const pemBlockCertificate = "CERTIFICATE"
