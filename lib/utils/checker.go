/*
Copyright 2019 Gravitational, Inc.

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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
)

// CertChecker is a drop-in replacement for ssh.CertChecker that also checks
// if the certificate (or key) were generated with a valid algorithm.
type CertChecker struct {
	ssh.CertChecker
}

// Authenticate checks the validity of a user certificate.
// a value for ServerConfig.PublicKeyCallback.
func (c *CertChecker) Authenticate(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	err := validate(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	perms, err := c.CertChecker.Authenticate(conn, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return perms, nil
}

// CheckCert checks certificate metadata and signature.
func (c *CertChecker) CheckCert(principal string, cert *ssh.Certificate) error {
	err := validate(cert)
	if err != nil {
		return trace.Wrap(err)
	}

	return c.CertChecker.CheckCert(principal, cert)
}

// CheckHostKey checks the validity of a host certificate.
func (c *CertChecker) CheckHostKey(addr string, remote net.Addr, key ssh.PublicKey) error {
	err := validate(key)
	if err != nil {
		return trace.Wrap(err)
	}

	return c.CertChecker.CheckHostKey(addr, remote, key)
}

func validate(key ssh.PublicKey) error {
	switch cert := key.(type) {
	case *ssh.Certificate:
		err := validateAlgorithm(cert.Key)
		if err != nil {
			return trace.Wrap(err)
		}
		err = validateAlgorithm(cert.SignatureKey)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	default:
		return validateAlgorithm(key)
	}
}

func validateAlgorithm(key ssh.PublicKey) error {
	cryptoKey, ok := key.(ssh.CryptoPublicKey)
	if !ok {
		return trace.BadParameter("unable to determine underlying public key")
	}
	k, ok := cryptoKey.CryptoPublicKey().(*rsa.PublicKey)
	if !ok {
		return trace.BadParameter("only RSA keys supported")
	}
	if k.N.BitLen() != teleport.RSAKeySize {
		return trace.BadParameter("found %v-bit key, only %v-bit supported", k.N.BitLen(), teleport.RSAKeySize)
	}

	return nil
}

// CreateCertificate creates a valid 2048-bit RSA certificate.
func CreateCertificate(principal string, certType uint32) (*ssh.Certificate, ssh.Signer, error) {
	// Create RSA key for CA and certificate to be signed by CA.
	caKey, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	cert, certSigner, err := createCertificate(principal, certType, caKey, key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cert, certSigner, nil
}

// CreateEllipticCertificate creates a valid, but not supported, ECDSA
// SSH certificate. This certificate is used to make sure Teleport rejects
// such certificates.
func CreateEllipticCertificate(principal string, certType uint32) (*ssh.Certificate, ssh.Signer, error) {
	// Create ECDSA key for CA and certificate to be signed by CA.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	cert, certSigner, err := createCertificate(principal, certType, caKey, key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cert, certSigner, nil
}

// createCertificate creates a SSH certificate for the given key signed by the
// given CA key. This function exists here to allow easy key generation for
// some of the more core packages like "sshutils".
func createCertificate(principal string, certType uint32, caKey crypto.Signer, key crypto.Signer) (*ssh.Certificate, ssh.Signer, error) {
	// Create CA.
	caPublicKey, err := ssh.NewPublicKey(caKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	caSigner, err := ssh.NewSignerFromKey(caKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Create key.
	publicKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	keySigner, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Create certificate and signer.
	cert := &ssh.Certificate{
		KeyId:           principal,
		ValidPrincipals: []string{principal},
		Key:             publicKey,
		SignatureKey:    caPublicKey,
		ValidAfter:      uint64(time.Now().UTC().Add(-1 * time.Minute).Unix()),
		ValidBefore:     uint64(time.Now().UTC().Add(1 * time.Minute).Unix()),
		CertType:        certType,
	}
	err = cert.SignCert(rand.Reader, caSigner)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certSigner, err := ssh.NewCertSigner(cert, keySigner)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cert, certSigner, nil
}
