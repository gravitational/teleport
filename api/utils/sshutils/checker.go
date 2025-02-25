/*
Copyright 2019-2021 Gravitational, Inc.

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

package sshutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
)

// CertChecker is a drop-in replacement for ssh.CertChecker. In FIPS mode,
// checks if the certificate (or key) were generated with a supported algorithm.
type CertChecker struct {
	ssh.CertChecker

	// FIPS means in addition to checking the validity of the key or
	// certificate, also check that FIPS 140-2 algorithms were used.
	FIPS bool

	// OnCheckCert is called when validating host certificate.
	OnCheckCert func(*ssh.Certificate) error
}

// Authenticate checks the validity of a user certificate.
func (c *CertChecker) Authenticate(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	err := c.validateFIPS(key)
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
	err := c.validateFIPS(cert)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.CertChecker.CheckCert(principal, cert)
	if err != nil {
		return trace.Wrap(err)
	}

	if c.OnCheckCert != nil {
		if err := c.OnCheckCert(cert); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// CheckHostKey checks the validity of a host certificate.
func (c *CertChecker) CheckHostKey(addr string, remote net.Addr, key ssh.PublicKey) error {
	err := c.validateFIPS(key)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.CertChecker.CheckHostKey(addr, remote, key)
	if err != nil {
		return trace.Wrap(err)
	}

	if cert, ok := key.(*ssh.Certificate); ok && c.OnCheckCert != nil {
		if err := c.OnCheckCert(cert); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (c *CertChecker) validateFIPS(key ssh.PublicKey) error {
	// When not in FIPS mode, accept all algorithms and key sizes.
	if !c.FIPS {
		return nil
	}

	switch cert := key.(type) {
	case *ssh.Certificate:
		err := validateFIPSAlgorithm(cert.Key)
		if err != nil {
			return trace.Wrap(err)
		}
		err = validateFIPSAlgorithm(cert.SignatureKey)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	default:
		return validateFIPSAlgorithm(key)
	}
}

func validateFIPSAlgorithm(key ssh.PublicKey) error {
	cryptoKey, ok := key.(ssh.CryptoPublicKey)
	if !ok {
		return trace.BadParameter("unable to determine underlying public key")
	}
	switch k := cryptoKey.CryptoPublicKey().(type) {
	case *rsa.PublicKey:
		if k.N.BitLen() != constants.RSAKeySize {
			return trace.BadParameter("found %v-bit RSA key, only %v-bit supported", k.N.BitLen(), constants.RSAKeySize)
		}
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() && k.Curve != elliptic.P384() {
			params := k.Curve.Params()
			if params == nil {
				return trace.BadParameter("unable to determine curve of ECDSA public key")
			}
			return trace.BadParameter("found ECDSA key with curve %s, only P-256 and P-384 are supported", params.Name)
		}
	default:
		return trace.BadParameter("only RSA and ECDSA keys supported")
	}
	return nil
}
