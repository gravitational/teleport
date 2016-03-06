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
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gravitational/teleport"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

// ListenAndServeTLS sets up TLS listener for the http handler
// and blocks in listening and serving requests
func ListenAndServeTLS(address string, handler http.Handler,
	certFile, keyFile string) error {

	tlsConfig, err := CreateTLSConfiguration(certFile, keyFile)
	if err != nil {
		return trace.Wrap(err)
	}

	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	return http.Serve(listener, handler)
}

// CreateTLSConfiguration sets up default TLS configuration
func CreateTLSConfiguration(certFile, keyFile string) (*tls.Config, error) {
	config := &tls.Config{}

	if _, err := os.Stat(certFile); err != nil {
		return nil, trace.Wrap(teleport.BadParameter("certificate", fmt.Sprintf("certificate is not accessible by '%v'", certFile)))
	}
	if _, err := os.Stat(keyFile); err != nil {
		return nil, trace.Wrap(teleport.BadParameter("certificate", fmt.Sprintf("certificate is not accessible by '%v'", certFile)))
	}

	log.Infof("[PROXY] TLS cert=%v key=%v", certFile, keyFile)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config.Certificates = []tls.Certificate{cert}

	config.CipherSuites = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,

		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,

		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	}

	config.MinVersion = tls.VersionTLS12
	config.SessionTicketsDisabled = false
	config.ClientSessionCache = tls.NewLRUClientSessionCache(
		DefaultLRUCapacity)

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

// GenerateSelfSignedCert generates a self signed certificate that
// is valid for given domain names and ips, returns PEM-encoded bytes with key and cert
func GenerateSelfSignedCert(domainNames []string, IPAddresses []string) (*TLSCredentials, error) {
	ips := make([]net.IP, len(IPAddresses))
	for i, addr := range IPAddresses {
		ip := net.ParseIP(addr)
		if ip == nil {
			return nil, trace.Wrap(
				teleport.BadParameter("ip", fmt.Sprintf("%v is not a valid IP", addr)))
		}
		ips[i] = ip
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(DefaultCertTTL)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Teleport"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign

	template.DNSNames = append(template.DNSNames, domainNames...)
	template.IPAddresses = append(template.IPAddresses, ips...)

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,       // template of the certificate
		&template,       // template of the CA certificate
		&priv.PublicKey, // public key of the signee
		priv)            // private key of the signer
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

const (
	// DefaultLRUCapacity is a capacity for LRU session cache
	DefaultLRUCapacity = 1024
	// DefaultCertTTL sets the TTL of the self-signed certificate (1 year)
	DefaultCertTTL = (24 * time.Hour) * 365
)
