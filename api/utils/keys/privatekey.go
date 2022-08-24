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

// Package keys defines common interfaces for Teleport client keys.
package keys

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/gravitational/teleport/api/utils/sshutils/ppk"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	PKCS1PrivateKeyType = "RSA PRIVATE KEY"
	PKCS8PrivateKeyType = "PRIVATE KEY"
	ECPrivateKeyType    = "EC PRIVATE KEY"
)

// PrivateKey implements crypto.Signer with additional helper methods. The underlying
// private key may be a standard crypto.Signer implemented in the standard library
// (aka *rsa.PrivateKey, *ecdsa.PrivateKey, or ed25519.PrivateKey), or it may be a
// custom implementation for a non-standard private key, such as a hardware key.
type PrivateKey struct {
	Signer
	sshPub ssh.PublicKey
}

// NewPrivateKey returns a new PrivateKey for the given crypto.Signer.
func NewPrivateKey(signer Signer) (*PrivateKey, error) {
	sshPub, err := ssh.NewPublicKey(signer.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &PrivateKey{
		Signer: signer,
		sshPub: sshPub,
	}, nil
}

// SSHPublicKey returns the ssh.PublicKey representiation of the public key.
func (k *PrivateKey) SSHPublicKey() ssh.PublicKey {
	return k.sshPub
}

// SSHPublicKey returns the ssh.PublicKey representiation of the public key.
func (k *PrivateKey) MarshalSSHPublicKey() []byte {
	return ssh.MarshalAuthorizedKey(k.sshPub)
}

// agentKeyComment is used to generate an agent key comment.
type agentKeyComment struct {
	user string
}

func (a *agentKeyComment) String() string {
	return fmt.Sprintf("teleport:%s", a.user)
}

// AsAgentKey converts PrivateKey to a agent.AddedKey. If the given PrivateKey is not
// supported as an agent key, a trace.NotImplemented error is returned.
func (k *PrivateKey) AsAgentKey(sshCert *ssh.Certificate) (agent.AddedKey, error) {
	signer, ok := k.Signer.(*StandardSigner)
	if !ok {
		// We return a not implemented error because agent.AddedKey only
		// supports plain RSA, ECDSA, and ED25519 keys. Non-standard private
		// keys, like hardware-based private keys, will require custom solutions
		// which may not be included in their initial implementation. This will
		// only affect functionality related to agent forwarding, so we give the
		// caller the ability to handle the error gracefully.
		return agent.AddedKey{}, trace.NotImplemented("cannot create an agent key using private key signer of type %T", k.Signer)
	}

	// put a teleport identifier along with the teleport user into the comment field
	comment := agentKeyComment{user: sshCert.KeyId}
	return agent.AddedKey{
		PrivateKey:       signer.Signer,
		Certificate:      sshCert,
		Comment:          comment.String(),
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}, nil
}

// PPKFile returns a PuTTY PPK-formatted keypair
func (k *PrivateKey) PPKFile() ([]byte, error) {
	signer, ok := k.Signer.(*StandardSigner)
	if !ok {
		return nil, trace.BadParameter("cannot use private key of type %T as rsa.PrivateKey", k)
	}
	rsaKey, ok := signer.Signer.(*rsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("cannot use private key of type %T as rsa.PrivateKey", k)
	}
	ppkFile, err := ppk.ConvertToPPK(rsaKey, k.MarshalSSHPublicKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ppkFile, nil
}

// RSAPrivateKeyPEM returns a PEM encoded RSA private key for the given key.
// If the given key is not an RSA key, then an error will be returned.
//
// This is used by some integrations which currently only support raw RSA private keys,
// like Kubernetes, MongoDB, and PPK files for windows.
func (k *PrivateKey) RSAPrivateKeyPEM() ([]byte, error) {
	signer := k.GetBaseSigner()
	if _, ok := signer.(*rsa.PrivateKey); !ok {
		return nil, trace.BadParameter("cannot get rsa key PEM for private key of type %T", signer)
	}
	return k.PrivateKeyPEM(), nil
}

// GetBaseSigner is a helper method to return the actual nested crypto.Signer for this PrivateKey.
func (k *PrivateKey) GetBaseSigner() crypto.Signer {
	switch signer := k.Signer.(type) {
	case *StandardSigner:
		return signer.Signer
	default:
		return signer
	}
}

// LoadPrivateKey returns the PrivateKey for the given key file.
func LoadPrivateKey(keyFile string) (*PrivateKey, error) {
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	priv, err := ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return priv, nil
}

// ParsePrivateKey returns the PrivateKey for the given key PEM block.
func ParsePrivateKey(keyPEM []byte) (*PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, trace.BadParameter("expected PEM encoded private key")
	}

	switch block.Type {
	case PKCS1PrivateKeyType:
		cryptoSigner, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewPrivateKey(newStandardSigner(cryptoSigner, keyPEM))
	case ECPrivateKeyType:
		cryptoSigner, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewPrivateKey(newStandardSigner(cryptoSigner, keyPEM))
	case PKCS8PrivateKeyType:
		priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cryptoSigner, ok := priv.(crypto.Signer)
		if !ok {
			return nil, trace.BadParameter("x509.ParsePKCS8PrivateKey returned an invalid private key of type %T", priv)
		}
		return NewPrivateKey(newStandardSigner(cryptoSigner, keyPEM))
	default:
		return nil, trace.BadParameter("unexpected private key PEM type %q", block.Type)
	}
}

// LoadKeyPair returns the PrivateKey for the given private and public key files.
func LoadKeyPair(privFile, sshPubFile string) (*PrivateKey, error) {
	privPEM, err := os.ReadFile(privFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	marshalledSSHPub, err := os.ReadFile(sshPubFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	priv, err := ParseKeyPair(privPEM, marshalledSSHPub)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return priv, nil
}

// ParseKeyPair returns the PrivateKey for the given private and public key PEM blocks.
func ParseKeyPair(privPEM, marshalledSSHPub []byte) (*PrivateKey, error) {
	priv, err := ParsePrivateKey(privPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify that the private key's public key matches the expected public key.
	if !bytes.Equal(ssh.MarshalAuthorizedKey(priv.SSHPublicKey()), marshalledSSHPub) {
		return nil, trace.CompareFailed("the given private and public keys do not form a valid keypair")
	}

	return priv, nil
}

// LoadX509KeyPair parse a tls.Certificate from a private key file and certificate file.
// This should be used instead of tls.LoadX509KeyPair to support non-raw private keys, like PIV keys.
func LoadX509KeyPair(certFile, keyFile string) (tls.Certificate, error) {
	keyPEMBlock, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, trace.ConvertSystemError(err)
	}

	certPEMBlock, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, trace.ConvertSystemError(err)
	}

	tlsCert, err := X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return tlsCert, nil
}

// X509KeyPair parse a tls.Certificate from a private key PEM and certificate PEM.
// This should be used instead of tls.X509KeyPair to support non-raw private keys, like PIV keys.
func X509KeyPair(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
	priv, err := ParsePrivateKey(keyPEMBlock)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	tlsCert, err := priv.TLSCertificate(certPEMBlock)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return tlsCert, nil
}
