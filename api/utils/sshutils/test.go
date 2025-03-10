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

package sshutils

import (
	"crypto/rand"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/cryptopatch"
)

const defaultPrincipal = "127.0.0.1"

// MakeTestSSHCA generates a new SSH certificate authority for tests.
func MakeTestSSHCA() (ssh.Signer, error) {
	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, nil
}

// MakeSpoofedHostCert makes an SSH host certificate that claims to be signed
// by the provided CA but in fact is signed by a different CA.
func MakeSpoofedHostCert(realCA ssh.Signer) (ssh.Signer, error) {
	fakeCA, err := MakeTestSSHCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return makeHostCert(realCA.PublicKey(), fakeCA, defaultPrincipal)
}

// MakeRealHostCert makes an SSH host certificate that is signed by the
// provided CA.
func MakeRealHostCert(realCA ssh.Signer) (ssh.Signer, error) {
	return makeHostCert(realCA.PublicKey(), realCA, defaultPrincipal)
}

// MakeRealHostCertWithPrincipals makes an SSH host certificate that is signed by the
// provided CA for the provided principals.
func MakeRealHostCertWithPrincipals(realCA ssh.Signer, principals ...string) (ssh.Signer, error) {
	return makeHostCert(realCA.PublicKey(), realCA, principals...)
}

func makeHostCert(signKey ssh.PublicKey, signer ssh.Signer, principals ...string) (ssh.Signer, error) {
	priv, err := cryptopatch.GenerateRSAKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privSigner, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return nil, trace.Wrap(err)
	}

	cert := &ssh.Certificate{
		Nonce:           nonce,
		Key:             pub,
		CertType:        ssh.HostCert,
		SignatureKey:    signKey,
		ValidPrincipals: principals,
		ValidBefore:     uint64(time.Now().Add(time.Hour).Unix()),
	}

	// We cannot use ssh.Certificate SignCert method since we're intentionally
	// setting invalid signature key to make a spoofed cert in some tests.
	//
	// When marshaling cert for signing, last 4 bytes containing trailing
	// signature length are dropped:
	//
	// https://cs.opensource.google/go/x/crypto/+/32db7946:ssh/certs.go;l=456-462
	bytesForSigning := cert.Marshal()
	bytesForSigning = bytesForSigning[:len(bytesForSigning)-4]

	cert.Signature, err = signer.Sign(rand.Reader, bytesForSigning)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certSigner, err := ssh.NewCertSigner(cert, privSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certSigner, nil
}
