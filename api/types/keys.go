/*
Copyright 2021 Gravitational, Inc.

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

package types

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

// ParsePrivateKey parses a PEM encoded private key and returns a
// crypto.Signer. Only supports RSA private keys.
func ParsePrivateKey(bytes []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("failed to decode private key PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, trace.BadParameter("unsupported private key type %q", block.Type)
	}
}

// ParsePublicKey parses a PEM encoded public key and returns a
// crypto.PublicKey. Only support RSA public keys.
func ParsePublicKey(bytes []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("failed to decode public key PEM block")
	}

	switch block.Type {
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		return nil, trace.BadParameter("unsupported public key type %q", block.Type)
	}
}

// GenerateSelfSignedSigningCert generates self-signed certificate used for digital signatures
func GenerateSelfSignedSigningCert(entity pkix.Name, dnsNames []string, ttl time.Duration) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
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
