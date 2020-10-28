/*
Copyright 2017 Gravitational, Inc.

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

package tlsca

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

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ClusterName returns cluster name from organization
func ClusterName(subject pkix.Name) (string, error) {
	if len(subject.Organization) == 0 {
		return "", trace.BadParameter("missing subject organization")
	}
	return subject.Organization[0], nil
}

// GenerateSelfSignedCA generates self-signed certificate authority used for internal inter-node communications
func GenerateSelfSignedCAWithPrivateKey(priv *rsa.PrivateKey, entity pkix.Name, dnsNames []string, ttl time.Duration) ([]byte, []byte, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(ttl)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// this is important, otherwise go will accept certificate authorities
	// signed by the same private key and having the same subject (happens in tests)
	entity.SerialNumber = serialNumber.String()

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Issuer:                entity,
		Subject:               entity,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
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

// GenerateSelfSignedCA generates self-signed certificate authority used for internal inter-node communications
func GenerateSelfSignedCA(entity pkix.Name, dnsNames []string, ttl time.Duration) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return GenerateSelfSignedCAWithPrivateKey(priv, entity, dnsNames, ttl)
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
func GenerateCertificateRequestPEM(subject pkix.Name, privateKeyBytes []byte) ([]byte, error) {
	privateKey, err := ssh.ParseRawPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr := &x509.CertificateRequest{
		Subject: subject,
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, privateKey)
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
				return nil, trace.BadParameter("failed parsing private key")
			}
		}
	}

	switch k := generalKey.(type) {
	case *rsa.PrivateKey:
		return k, nil
	case *ecdsa.PrivateKey:
		return k, nil
	}

	return nil, trace.BadParameter("unsupported private key type")
}

// ParsePublicKeyPEM parses public key PEM
func ParsePublicKeyPEM(bytes []byte) (interface{}, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("expected PEM-encoded block")
	}
	return ParsePublicKeyDER(block.Bytes)
}

// ParsePublicKeyDER parses unencrypted DER-encoded publice key
func ParsePublicKeyDER(der []byte) (crypto.PublicKey, error) {
	generalKey, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return generalKey, nil
}

// MarshalPublicKeyFromPrivateKeyPEM extracts public key from private key
// and returns PEM marshalled key
func MarshalPublicKeyFromPrivateKeyPEM(privateKey crypto.PrivateKey) ([]byte, error) {
	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("expected RSA key")
	}
	rsaPublicKey := rsaPrivateKey.Public()
	derBytes, err := x509.MarshalPKIXPublicKey(rsaPublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: derBytes}), nil
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
