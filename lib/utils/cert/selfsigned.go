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

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// macMaxTLSCertValidityPeriod is the maximum validity period
// for a TLS certificate enforced by macOS.
// As of Go 1.18, certificates are validated via the system
// verifier and not in Go.
const macMaxTLSCertValidityPeriod = 825 * 24 * time.Hour

// Credentials keeps the typical 3 components of a proper TLS configuration
type Credentials struct {
	// PublicKey in PEM format
	PublicKey []byte
	// PrivateKey in PEM format
	PrivateKey []byte
	Cert       []byte
}

// GenerateSelfSignedCert generates a self-signed certificate that
// is valid for given domain names and IPs. If extended key usage
// is not specified, the cert will be generated for server auth.
func GenerateSelfSignedCert(hostNames []string, ipAddresses []string, eku ...x509.ExtKeyUsage) (*Credentials, error) {
	if len(eku) == 0 {
		// if not specified, assume this cert is for server auth,
		// which is required for validation on macOS:
		// https://support.apple.com/en-in/HT210176
		eku = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}

	priv, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
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
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           eku,
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
		ipParsed := net.ParseIP(ip)
		if ipParsed == nil {
			return nil, trace.Errorf("Unable to parse IP address for self-signed certificate (%v)", ip)
		}
		template.IPAddresses = append(template.IPAddresses, ipParsed)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKeyBytes, err := keys.MarshalPrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publicKeyBytes, err := keys.MarshalPublicKey(priv.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Credentials{
		PrivateKey: privateKeyBytes,
		PublicKey:  publicKeyBytes,
		Cert:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}),
	}, nil
}
