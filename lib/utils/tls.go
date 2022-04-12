/*
Copyright 2015 Gravitational, Inc.

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// TLSConfig returns default TLS configuration strong defaults.
func TLSConfig(cipherSuites []uint16) *tls.Config {
	config := &tls.Config{}
	SetupTLSConfig(config, cipherSuites)
	return config
}

// SetupTLSConfig sets up cipher suites in existing TLS config
func SetupTLSConfig(config *tls.Config, cipherSuites []uint16) {
	// If ciphers suites were passed in, use them. Otherwise use the the
	// Go defaults.
	if len(cipherSuites) > 0 {
		config.CipherSuites = cipherSuites
	}

	config.MinVersion = tls.VersionTLS12
	config.SessionTicketsDisabled = false
	config.ClientSessionCache = tls.NewLRUClientSessionCache(DefaultLRUCapacity)
}

// CreateTLSConfiguration sets up default TLS configuration
func CreateTLSConfiguration(certFile, keyFile string, cipherSuites []uint16) (*tls.Config, error) {
	config := TLSConfig(cipherSuites)

	if _, err := os.Stat(certFile); err != nil {
		return nil, trace.BadParameter("certificate is not accessible by '%v'", certFile)
	}
	if _, err := os.Stat(keyFile); err != nil {
		return nil, trace.BadParameter("certificate is not accessible by '%v'", certFile)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.Certificates = []tls.Certificate{cert}

	return config, nil
}

// TLSCredentials keeps the typical 3 components of a proper HTTPS configuration
type TLSCredentials struct {
	// PublicKey in PEM format
	PublicKey []byte
	// PrivateKey in PEM format
	PrivateKey []byte
	Cert       []byte
}

// macMaxTLSCertValidityPeriod is the maximum validitiy period
// for a TLS certificate enforced by macOS.
// As of Go 1.18, certificates are validated via the system
// verifier and not in Go.
const macMaxTLSCertValidityPeriod = 825 * 24 * time.Hour

// GenerateSelfSignedCert generates a self-signed certificate that
// is valid for given domain names and ips, returns PEM-encoded bytes with key and cert
func GenerateSelfSignedCert(hostNames []string) (*TLSCredentials, error) {
	priv, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
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
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}

	return &TLSCredentials{
		PublicKey:  pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: publicKeyBytes}),
		PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}),
		Cert:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}),
	}, nil
}

// CipherSuiteMapping transforms Teleport formatted cipher suites strings
// into uint16 IDs.
func CipherSuiteMapping(cipherSuites []string) ([]uint16, error) {
	out := make([]uint16, 0, len(cipherSuites))

	for _, cs := range cipherSuites {
		c, ok := cipherSuiteMapping[cs]
		if !ok {
			return nil, trace.BadParameter("cipher suite not supported: %v", cs)
		}

		out = append(out, c)
	}

	return out, nil
}

// cipherSuiteMapping is the mapping between Teleport formatted cipher
// suites strings and uint16 IDs.
var cipherSuiteMapping = map[string]uint16{
	"tls-rsa-with-aes-128-cbc-sha":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"tls-rsa-with-aes-256-cbc-sha":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"tls-rsa-with-aes-128-cbc-sha256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"tls-rsa-with-aes-128-gcm-sha256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"tls-rsa-with-aes-256-gcm-sha384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"tls-ecdhe-ecdsa-with-aes-128-cbc-sha":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"tls-ecdhe-ecdsa-with-aes-256-cbc-sha":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"tls-ecdhe-rsa-with-aes-128-cbc-sha":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"tls-ecdhe-rsa-with-aes-256-cbc-sha":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"tls-ecdhe-ecdsa-with-aes-128-cbc-sha256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"tls-ecdhe-rsa-with-aes-128-cbc-sha256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"tls-ecdhe-rsa-with-aes-128-gcm-sha256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"tls-ecdhe-ecdsa-with-aes-128-gcm-sha256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"tls-ecdhe-rsa-with-aes-256-gcm-sha384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"tls-ecdhe-ecdsa-with-aes-256-gcm-sha384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"tls-ecdhe-rsa-with-chacha20-poly1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"tls-ecdhe-ecdsa-with-chacha20-poly1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
}

const (
	// DefaultLRUCapacity is a capacity for LRU session cache
	DefaultLRUCapacity = 1024
	// DefaultCertTTL sets the TTL of the self-signed certificate (1 year)
	DefaultCertTTL = (24 * time.Hour) * 365
)

// DefaultCipherSuites returns the default list of cipher suites that
// Teleport supports. By default Teleport only support modern ciphers
// (Chacha20 and AES GCM) and key exchanges which support perfect forward
// secrecy (ECDHE).
//
// Note that TLS_RSA_WITH_AES_128_GCM_SHA{256,384} have been dropped due to
// being banned by HTTP2 which breaks GRPC clients. For more information see:
// https://tools.ietf.org/html/rfc7540#appendix-A. These two can still be
// manually added if needed.
func DefaultCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,

		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}
}
