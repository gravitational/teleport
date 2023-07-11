/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package cert

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/auth/native"
)

// macMaxTLSCertValidityPeriod is the maximum validity period
// for a TLS certificate enforced by macOS.
// As of Go 1.18, certificates are validated via the system
// verifier and not in Go.
const macMaxTLSCertValidityPeriod = 825 * 24 * time.Hour

// Credentials keeps the typical 3 components of a proper HTTPS configuration
type Credentials struct {
	// PublicKey in PEM format
	PublicKey []byte
	// PrivateKey in PEM format
	PrivateKey []byte
	Cert       []byte
}

// GenerateSelfSignedCert generates a self-signed certificate that
// is valid for given domain names and ips, returns PEM-encoded bytes with key and cert
func GenerateSelfSignedCert(hostNames []string, ipAddresses ...string) (*Credentials, error) {
	priv, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(macMaxTLSCertValidityPeriod)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	entity := pkix.Name{
		CommonName:   "localhost",
		Country:      []string{"US"},
		Organization: []string{"localhost"},
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Issuer:                entity,
		Subject:               entity,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// collect IP addresses localhost resolves to and add them to the cert. template:
	template.DNSNames = append(hostNames, "localhost.local")
	ips, _ := net.LookupIP("localhost")
	if ips != nil {
		template.IPAddresses = append(ips, net.ParseIP("::1"))
	}
	for _, ip := range ipAddresses {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(ip))
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		logrus.Error(err)
		return nil, trace.Wrap(err)
	}

	return &Credentials{
		PublicKey:  pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: publicKeyBytes}),
		PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}),
		Cert:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}),
	}, nil
}
