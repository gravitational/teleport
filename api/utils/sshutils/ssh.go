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

// Package sshutils defines several functions and types used across the
// Teleport API and other Teleport packages when working with SSH.
package sshutils

import (
	"crypto/subtle"
	"fmt"
	"net"
	"runtime"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ParseCertificate parses an SSH certificate from the authorized_keys format.
func ParseCertificate(buf []byte) (*ssh.Certificate, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, ok := k.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("not an SSH certificate")
	}

	return cert, nil
}

// ProxyClientSSHConfig returns an ssh.ClientConfig with SSH credentials from this
// Key and HostKeyCallback matching SSH CAs in the Key.
//
// The config is set up to authenticate to proxy with the first available principal.
//
func ProxyClientSSHConfig(sshCert, privKey []byte, caCerts [][]byte) (*ssh.ClientConfig, error) {
	cert, err := ParseCertificate(sshCert)
	if err != nil {
		return nil, trace.Wrap(err, "failed to extract username from SSH certificate")
	}

	authMethod, err := AsAuthMethod(cert, privKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert key pair to auth method")
	}

	hostKeyCallback, err := HostKeyCallback(caCerts)
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert certificate authorities to HostKeyCallback")
	}

	// The KeyId is not always a valid principal, so we use the first valid principal instead.
	user := cert.KeyId
	if len(cert.ValidPrincipals) > 0 {
		user = cert.ValidPrincipals[0]
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         defaults.DefaultDialTimeout,
	}, nil
}

// AsAuthMethod returns an "auth method" interface, a common abstraction
// used by Golang SSH library. This is how you actually use a Key to feed
// it into the SSH lib.
func AsAuthMethod(sshCert *ssh.Certificate, privKey []byte) (ssh.AuthMethod, error) {
	keys, err := AsAgentKeys(sshCert, privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := ssh.NewSignerFromKey(keys[0].PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if signer, err = ssh.NewCertSigner(keys[0].Certificate, signer); err != nil {
		return nil, trace.Wrap(err)
	}
	return NewAuthMethodForCert(signer), nil
}

// AsAgentKeys converts Key struct to a []*agent.AddedKey. All elements
// of the []*agent.AddedKey slice need to be loaded into the agent!
func AsAgentKeys(sshCert *ssh.Certificate, privKey []byte) ([]agent.AddedKey, error) {
	// unmarshal private key bytes into a *rsa.PrivateKey
	privateKey, err := ssh.ParseRawPrivateKey(privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// put a teleport identifier along with the teleport user into the comment field
	comment := fmt.Sprintf("teleport:%v", sshCert.KeyId)

	// On Windows, return the certificate with the private key embedded.
	if runtime.GOOS == constants.WindowsOS {
		return []agent.AddedKey{
			{
				PrivateKey:       privateKey,
				Certificate:      sshCert,
				Comment:          comment,
				LifetimeSecs:     0,
				ConfirmBeforeUse: false,
			},
		}, nil
	}

	// On Unix, return the certificate (with embedded private key) as well as
	// a private key.
	//
	// This is done because OpenSSH clients older than OpenSSH 7.3/7.3p1
	// (2016-08-01) have a bug in how they use certificates that have been loaded
	// in an agent. Specifically when you add a certificate to an agent, you can't
	// just embed the private key within the certificate, you have to add the
	// certificate and private key to the agent separately. Teleport works around
	// this behavior to ensure OpenSSH interoperability.
	//
	// For more details see the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
	// WARNING: callers expect the returned slice to be __exactly as it is__
	return []agent.AddedKey{
		{
			PrivateKey:       privateKey,
			Certificate:      sshCert,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
		{
			PrivateKey:       privateKey,
			Certificate:      nil,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
	}, nil
}

// HostKeyCallback returns an ssh.HostKeyCallback that validates host
// keys/certs against SSH CAs in the Key.
//
// If not CAs are present in the Key, the returned ssh.HostKeyCallback is nil.
// This causes golang.org/x/crypto/ssh to prompt the user to verify host key
// fingerprint (same as OpenSSH does for an unknown host).
func HostKeyCallback(caCerts [][]byte) (ssh.HostKeyCallback, error) {
	var trustedKeys []ssh.PublicKey
	for _, caCert := range caCerts {
		_, _, publicKey, _, _, err := ssh.ParseKnownHosts(caCert)
		if err != nil {
			return nil, trace.BadParameter("failed parsing CA cert: %v; raw CA cert line: %q", err, caCert)
		}
		trustedKeys = append(trustedKeys, publicKey)
	}
	// No CAs are provided, return a nil callback which will prompt the user
	// for trust.
	if len(trustedKeys) == 0 {
		return nil, nil
	}

	return func(host string, a net.Addr, hostKey ssh.PublicKey) error {
		clusterCert, ok := hostKey.(*ssh.Certificate)
		if ok {
			hostKey = clusterCert.SignatureKey
		}
		for _, trustedKey := range trustedKeys {
			if KeysEqual(trustedKey, hostKey) {
				return nil
			}
		}
		return trace.AccessDenied("host %v is untrusted or Teleport CA has been rotated; try getting new credentials by logging in again ('tsh login') or re-exporting the identity file ('tctl auth sign' or 'tsh login -o'), depending on how you got them initially", host)
	}, nil
}

// CertAuthMethod is a wrapper around ssh.Signer (certificate signer) object.
// CertAuthMethod then implements ssh.AuthMethod interface around this one certificate signer.
//
// We need this wrapper because Golang's SSH library's unfortunate API design. It uses
// callbacks with 'authMethod' interfaces and without this wrapper it is impossible to
// tell which certificate an 'authMethod' passed via a callback had succeeded authenticating with.
type CertAuthMethod struct {
	ssh.AuthMethod
	Cert ssh.Signer
}

func NewAuthMethodForCert(cert ssh.Signer) *CertAuthMethod {
	return &CertAuthMethod{
		Cert: cert,
		AuthMethod: ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return []ssh.Signer{cert}, nil
		}),
	}
}

// KeysEqual is constant time compare of the keys to avoid timing attacks
func KeysEqual(ak, bk ssh.PublicKey) bool {
	a := ssh.Marshal(ak)
	b := ssh.Marshal(bk)
	return (len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1)
}
