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

package tlsca

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/utils"
)

// ClusterName returns cluster name from organization
func ClusterName(subject pkix.Name) (string, error) {
	if len(subject.Organization) == 0 {
		return "", trace.BadParameter("missing subject organization")
	}
	return subject.Organization[0], nil
}

// GenerateSelfSignedCAWithSigner generates self-signed certificate authority used for internal inter-node communications
func GenerateSelfSignedCAWithSigner(signer crypto.Signer, entity pkix.Name, dnsNames []string, ttl time.Duration) ([]byte, error) {
	return GenerateSelfSignedCAWithConfig(GenerateCAConfig{
		Signer:   signer,
		Entity:   entity,
		DNSNames: dnsNames,
		TTL:      ttl,
		Clock:    clockwork.NewRealClock(),
	})
}

// GenerateCAConfig defines the configuration for generating
// self-signed CA certificates
type GenerateCAConfig struct {
	Signer      crypto.Signer
	Entity      pkix.Name
	DNSNames    []string
	IPAddresses []net.IP
	TTL         time.Duration
	Clock       clockwork.Clock
}

// setDefaults imposes defaults on this configuration
func (r *GenerateCAConfig) setDefaults() {
	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}
}

// GenerateSelfSignedCAWithConfig generates a new CA certificate from the specified
// configuration.
// Returns PEM-encoded private key/certificate payloads upon success
func GenerateSelfSignedCAWithConfig(config GenerateCAConfig) (certPEM []byte, err error) {
	config.setDefaults()
	notBefore := config.Clock.Now()
	notAfter := notBefore.Add(config.TTL)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this is important, otherwise go will accept certificate authorities
	// signed by the same private key and having the same subject (happens in tests)
	config.Entity.SerialNumber = serialNumber.String()

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Issuer:       config.Entity,
		Subject:      config.Entity,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		// Note: KeyUsageCRLSign is set only to generate empty CRLs for Desktop
		// Access authentication with Windows.
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              config.DNSNames,
		IPAddresses:           config.IPAddresses,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, config.Signer.Public(), config.Signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return certPEM, nil
}

// GenerateSelfSignedCA generates self-signed certificate authority used for internal inter-node communications
func GenerateSelfSignedCA(entity pkix.Name, dnsNames []string, ttl time.Duration) ([]byte, []byte, error) {
	priv, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certPEM, err := GenerateSelfSignedCAWithSigner(priv, entity, dnsNames, ttl)
	return keyPEM, certPEM, err
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

// GenerateCertificateRequestPEM returns PEM-encoded certificate signing
// request from the provided subject and private key.
func GenerateCertificateRequestPEM(subject pkix.Name, priv crypto.Signer) ([]byte, error) {
	csr := &x509.CertificateRequest{
		Subject: subject,
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	}), nil
}

// ParseCertificatePEM parses PEM-encoded certificate
func ParseCertificatePEM(bytes []byte) (*x509.Certificate, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing PEM encoded block")
	}
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

// ParseCertificatePEM parses multiple PEM-encoded certificates
func ParseCertificatePEMs(bytes []byte) ([]*x509.Certificate, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing PEM encoded block")
	}
	var blocks []*pem.Block
	block, remaining := pem.Decode(bytes)
	for block != nil {
		blocks = append(blocks, block)
		block, remaining = pem.Decode(remaining)
	}
	var certs []*x509.Certificate
	for _, block := range blocks {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// MarshalPublicKeyFromPrivateKeyPEM extracts public key from private key
// and returns PEM marshaled key
func MarshalPublicKeyFromPrivateKeyPEM(privateKey crypto.PrivateKey) ([]byte, error) {
	// TODO(nklaassen): DELETE IN 18.0.0 when this quirk is no longer necessary because all parsers can handle
	// either format.
	if rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey); ok {
		// This is weird and historical: we're marshaling an RSA public key into PKIX DER format and then
		// putting it into an "RSA PUBLIC KEY" PEM block. Normally RSA keys should either be:
		// - PKCS#1 DER format in an "RSA PUBLIC KEY" PEM block
		// - PKIX DER format in a "PUBLIC KEY" PEM block
		derBytes, err := x509.MarshalPKIXPublicKey(rsaPrivateKey.Public())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: keys.PKCS1PublicKeyType, Bytes: derBytes}), nil
	}
	// All private keys in the standard library implement crypto.Signer, which gives access to the public key.
	if signer, ok := privateKey.(crypto.Signer); ok {
		return keys.MarshalPublicKey(signer.Public())
	}
	return nil, trace.BadParameter("unsupported key type %T", privateKey)
}

// MarshalCertificatePEM takes a *x509.Certificate and returns the PEM
// encoded bytes.
func MarshalCertificatePEM(cert *x509.Certificate) ([]byte, error) {
	var buf bytes.Buffer

	err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

// CalculatePins returns the SPKI pins for the given set of concatenated
// PEM-encoded certificates
func CalculatePins(certsBytes []byte) ([]string, error) {
	certs, err := ParseCertificatePEMs(certsBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pins := make([]string, 0, len(certs))
	for _, cert := range certs {
		pins = append(pins, utils.CalculateSPKI(cert))
	}
	return pins, nil
}
