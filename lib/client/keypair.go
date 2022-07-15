/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"crypto"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"runtime"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/sshutils/ppk"
	"github.com/gravitational/teleport/lib/auth/native"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/trace"
)

type KeyPair interface {
	PrivateKey() crypto.PrivateKey

	PublicKeyPEM() []byte

	Signer() crypto.Signer

	AsAgentKeys(sshCert *ssh.Certificate) []agent.AddedKey

	// PrivateKeyPEM is data about a private key that we want to store on disk.
	// This can be data necessary to retrieve the key, such as a yubikey card name
	// and slot, or it can be a full PEM encoded private key.
	PrivateKeyData() []byte

	PPKFile() ([]byte, error)

	TLSCertificate(certRaw []byte) (tls.Certificate, error)

	Equals(KeyPair) bool

	// TODO: nontrivial to remove these remaining usages
	PrivateKeyPEMTODO() []byte
}

// NewKeyPair returns a new KeyPair for the given private key data and public key PEM.
// For non-rsa keys, the privateKeyData is used to identity where we can get the key
// data from, such as a specific yubikey card and slot.
func NewKeyPair(privateKeyData, pubPEM []byte) KeyPair {
	// TODO: handle other privateKeyData types
	return ParseRSAKeyPair(privateKeyData, pubPEM)
}

type RSAKeyPair struct {
	rsaPrivateKey *rsa.PrivateKey
	rsaPublicKey  *rsa.PublicKey
	privateKeyPEM []byte
	publicKeyPEM  []byte
}

func GenerateRSAKeyPair() (*RSAKeyPair, error) {
	priv, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RSAKeyPair{
		rsaPrivateKey: priv,
	}, nil
}

// Retuns a new RSAKeyPair from an existing PEM-encoded RSA key pair.
func ParseRSAKeyPair(priv, pub []byte) *RSAKeyPair {
	privPEM, _ := pem.Decode(priv)
	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(privPEM.Bytes)
	if err != nil {
		// TODO: handle error
		panic(err)
	}

	return &RSAKeyPair{
		rsaPrivateKey: rsaPrivateKey,
		// TODO: check pub vs priv?
		privateKeyPEM: priv,
		publicKeyPEM:  pub,
	}
}

func (r *RSAKeyPair) PrivateKey() crypto.PrivateKey {
	return r.rsaPrivateKey
}

func (r *RSAKeyPair) Signer() crypto.Signer {
	return r.rsaPrivateKey
}

func (r *RSAKeyPair) PublicKeyPEM() []byte {
	pub, err := ssh.NewPublicKey(&r.rsaPrivateKey.PublicKey)
	if err != nil {
		// TODO: handle error
		panic(err)
	}
	return ssh.MarshalAuthorizedKey(pub)
}

func (r *RSAKeyPair) PrivateKeyPEMTODO() []byte {
	return r.PrivateKeyData()
}

func (r *RSAKeyPair) PrivateKeyData() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(r.rsaPrivateKey),
	})
}

// PPKFile returns a PuTTY PPK-formatted keypair
func (r *RSAKeyPair) PPKFile() ([]byte, error) {
	ppkFile, err := ppk.ConvertToPPK(r.privateKeyPEM, r.publicKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ppkFile, nil
}

func (r *RSAKeyPair) TLSCertificate(certRaw []byte) (tls.Certificate, error) {
	tlsCert, err := tls.X509KeyPair(certRaw, r.privateKeyPEM)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return tlsCert, nil
}

func (r *RSAKeyPair) Equals(other KeyPair) bool {
	rsa, ok := other.(*RSAKeyPair)
	if !ok {
		return false
	}

	return subtle.ConstantTimeCompare(r.privateKeyPEM, rsa.privateKeyPEM) == 1
}

// AsAgentKeys converts Key struct to a []*agent.AddedKey. All elements
// of the []*agent.AddedKey slice need to be loaded into the agent!
func (r *RSAKeyPair) AsAgentKeys(sshCert *ssh.Certificate) []agent.AddedKey {
	// put a teleport identifier along with the teleport user into the comment field
	comment := fmt.Sprintf("teleport:%v", sshCert.KeyId)

	// On all OS'es, return the certificate with the private key embedded.
	agents := []agent.AddedKey{
		{
			PrivateKey:       r.rsaPrivateKey,
			Certificate:      sshCert,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
	}

	if runtime.GOOS != constants.WindowsOS {
		// On Unix, return the certificate (with embedded private key) as well as
		// a private key.
		//
		// (2016-08-01) have a bug in how they use certificates that have been lo
		// This is done because OpenSSH clients older than OpenSSH 7.3/7.3p1aded
		// in an agent. Specifically when you add a certificate to an agent, you can't
		// just embed the private key within the certificate, you have to add the
		// certificate and private key to the agent separately. Teleport works around
		// this behavior to ensure OpenSSH interoperability.
		//
		// For more details see the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
		// WARNING: callers expect the returned slice to be __exactly as it is__

		agents = append(agents, agent.AddedKey{
			PrivateKey:       r.rsaPrivateKey,
			Certificate:      nil,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		})
	}

	return agents
}
