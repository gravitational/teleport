/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"os"
	"time"

	"github.com/gravitational/trace"
)

// MTLSCerts is the result for mTLS struct generator
type MTLSCerts struct {
	// caCert is a CA certificate struct used to generate mTLS CA cert and private key
	caCert x509.Certificate
	// clientCert is a certificate struct used to generate mTLS server cert and private key
	serverCert x509.Certificate
	// clientCert is a certificate struct used to generate mTLS client cert and private key
	clientCert x509.Certificate
	// CACert is the resulting CA certificate and private key
	CACert *keyPair
	// ServerCert is the resulting server certificate and private key
	ServerCert *keyPair
	// ClientCert is the resulting client certificate and private key
	ClientCert *keyPair
}

// keyPair is the pair of certificate and private key
type keyPair struct {
	// PrivateKey represents certificate private key
	PrivateKey *rsa.PrivateKey
	// Certificate represents certificate
	Certificate []byte
}

// GenerateMTLSCerts creates new MTLS certificate generator
func GenerateMTLSCerts(dnsNames []string, ips []string, ttl time.Duration, length int) (*MTLSCerts, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(ttl)

	caDistinguishedName := pkix.Name{
		CommonName: "CA",
	}
	serverDistinguishedName := pkix.Name{
		CommonName: "Server",
	}
	clientDistinguishedName := pkix.Name{
		CommonName: "Client",
	}

	c := &MTLSCerts{
		caCert: x509.Certificate{
			Issuer:                caDistinguishedName,
			Subject:               caDistinguishedName,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			IsCA:                  true,
			MaxPathLenZero:        true,
			KeyUsage:              x509.KeyUsageCRLSign | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
		},
		clientCert: x509.Certificate{
			Issuer:      caDistinguishedName,
			Subject:     clientDistinguishedName,
			NotBefore:   notBefore,
			NotAfter:    notAfter,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			KeyUsage:    x509.KeyUsageDigitalSignature,
		},
		serverCert: x509.Certificate{
			Issuer:      caDistinguishedName,
			Subject:     serverDistinguishedName,
			NotBefore:   notBefore,
			NotAfter:    notAfter,
			KeyUsage:    x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
	}

	// Generate and assign serial numbers
	sn, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.caCert.SerialNumber = sn

	sn, err = rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.clientCert.SerialNumber = sn

	sn, err = rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.serverCert.SerialNumber = sn

	// Append SANs and IPs to Server and Client certs
	if err := c.appendSANs(&c.serverCert, dnsNames, ips); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := c.appendSANs(&c.clientCert, dnsNames, ips); err != nil {
		return nil, trace.Wrap(err)
	}

	// Run the generator
	err = c.generate(length)
	if err != nil {
		return c, err
	}

	return c, nil
}

// appendSANs appends subjectAltName hosts and IPs
func (c MTLSCerts) appendSANs(cert *x509.Certificate, dnsNames []string, ips []string) error {
	cert.DNSNames = dnsNames

	if len(ips) == 0 {
		for _, name := range dnsNames {
			ips, err := net.LookupIP(name)
			if err != nil {
				return trace.Wrap(err)
			}

			if ips != nil {
				cert.IPAddresses = append(cert.IPAddresses, ips...)
			}
		}
	} else {
		for _, ip := range ips {
			cert.IPAddresses = append(cert.IPAddresses, net.ParseIP(ip))
		}
	}

	return nil
}

// Generate generates CA, server and client certificates
func (c *MTLSCerts) generate(length int) error {
	caPK, caCertBytes, err := c.genCertAndPK(length, &c.caCert, nil, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	c.CACert = &keyPair{caPK, caCertBytes}

	serverPK, serverCertBytes, err := c.genCertAndPK(length, &c.serverCert, &c.caCert, caPK)
	if err != nil {
		return trace.Wrap(err)
	}
	c.ServerCert = &keyPair{serverPK, serverCertBytes}

	clientPK, clientCertBytes, err := c.genCertAndPK(length, &c.clientCert, &c.caCert, caPK)
	if err != nil {
		return trace.Wrap(err)
	}
	c.ClientCert = &keyPair{clientPK, clientCertBytes}

	return nil
}

// genCertAndPK generates and returns certificate and primary key
func (c *MTLSCerts) genCertAndPK(length int, cert *x509.Certificate, parent *x509.Certificate, signer *rsa.PrivateKey) (*rsa.PrivateKey, []byte, error) {
	// Generate PK
	pk, err := rsa.GenerateKey(rand.Reader, length)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Check if it's self-signed, assign signer and parent to self
	s := signer
	p := parent

	if s == nil {
		s = pk
	}

	if p == nil {
		p = cert
	}

	// Generate and sign cert
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, p, &pk.PublicKey, s)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return pk, certBytes, nil
}

// EncodeToMemory returns PEM config of certificate and private key
func (c *keyPair) EncodeToMemory(pwd string) ([]byte, []byte, error) {
	var err error

	pkBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(c.PrivateKey)}
	bytesPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate})

	// Encrypt with passphrase
	if pwd != "" {
		//nolint:staticcheck // deprecated, but we still need it to be encrypted because of fluentd requirements
		pkBlock, err = x509.EncryptPEMBlock(rand.Reader, pkBlock.Type, pkBlock.Bytes, []byte(pwd), x509.PEMCipherAES256)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	pkBytesPEM := pem.EncodeToMemory(pkBlock)

	return bytesPEM, pkBytesPEM, nil
}

// WriteFile writes certificate and key file, optionally encrypts private key with password if not empty
func (c *keyPair) WriteFile(certPath, keyPath, pwd string) error {
	bytesPEM, pkBytesPEM, err := c.EncodeToMemory(pwd)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.WriteFile(certPath, bytesPEM, perms)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.WriteFile(keyPath, pkBytesPEM, perms)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
