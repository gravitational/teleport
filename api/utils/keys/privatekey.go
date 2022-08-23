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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	pkcs8PrivateKeyType = "PRIVATE KEY"
	pkcs1PrivateKeyType = "RSA PRIVATE KEY"
	ecPrivateKeyType    = "EC PRIVATE KEY"
)

// PrivateKey implements crypto.Signer with additional helper methods.
type PrivateKey interface {
	crypto.Signer

	// PrivateKeyPEM returns PEM encoded private key data. This may be data necessary
	// to retrieve the key, such as a Yubikey serial number and slot, or it can be a
	// PKCS marshaled private key.
	//
	// The resulting PEM encoded data should only be decoded with ParsePrivateKey to
	// prevent errors from parsing non PKCS marshaled keys, such as a PIV key.
	PrivateKeyPEM() []byte

	// SSHPublicKey returns the ssh.PublicKey representiation of the public key.
	SSHPublicKey() ssh.PublicKey

	// TLSCertificate parses the given TLS certificate paired with the private key
	// to rerturn a tls.Certificate, ready to be used in a TLS handshake.
	TLSCertificate(tlsCert []byte) (tls.Certificate, error)
}

// LoadPrivateKey returns the PrivateKey for the given key file.
func LoadPrivateKey(keyFile string) (PrivateKey, error) {
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
func ParsePrivateKey(keyPEM []byte) (PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, trace.BadParameter("expected PEM encoded private key")
	}

	switch block.Type {
	case pkcs1PrivateKeyType:
		rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewRSAPrivateKey(rsaPrivateKey)
	case ecPrivateKeyType:
		ecdsaPrivateKey, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return newECDSAPrivateKey(ecdsaPrivateKey)
	case pkcs8PrivateKeyType:
		priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		switch priv := priv.(type) {
		case *rsa.PrivateKey:
			return NewRSAPrivateKey(priv)
		case *ecdsa.PrivateKey:
			return newECDSAPrivateKey(priv)
		case ed25519.PrivateKey:
			return newED25519(priv)
		default:
			return nil, trace.BadParameter("unknown private key type in PKCS#8 wrapping")
		}
	default:
		return nil, trace.BadParameter("unexpected private key PEM type %q", block.Type)
	}
}

// LoadKeyPair returns the PrivateKey for the given private and public key files.
func LoadKeyPair(privFile, sshPubFile string) (PrivateKey, error) {
	privPEM, err := os.ReadFile(privFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	sshPubPEM, err := os.ReadFile(sshPubFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	priv, err := ParseKeyPair(privPEM, sshPubPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return priv, nil
}

// ParseKeyPair returns the PrivateKey for the given private and public key PEM blocks.
func ParseKeyPair(privPEM, sshPubPEM []byte) (PrivateKey, error) {
	priv, err := ParsePrivateKey(privPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify that the private key's public key matches the expected public key.
	if !bytes.Equal(ssh.MarshalAuthorizedKey(priv.SSHPublicKey()), sshPubPEM) {
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

// GetRSAPrivateKeyPEM returns a PEM encoded RSA private key for the given key.
// If the given key is not an RSA key, then an error will be returned.
//
// This is used by some integrations which currently only support raw RSA private keys,
// like Kubernetes, MongoDB, and PPK files for windows.
func GetRSAPrivateKeyPEM(k PrivateKey) ([]byte, error) {
	if _, ok := k.(*RSAPrivateKey); !ok {
		return nil, trace.BadParameter("cannot get rsa key PEM for private key of type %T", k)
	}
	return k.PrivateKeyPEM(), nil
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
func AsAgentKey(priv PrivateKey, sshCert *ssh.Certificate) (agent.AddedKey, error) {
	var cryptoPriv crypto.PublicKey
	switch priv := priv.(type) {
	case *RSAPrivateKey:
		cryptoPriv = priv.PrivateKey
	case *ECDSAPrivateKey:
		cryptoPriv = priv.PrivateKey
	case *ED25519PrivateKey:
		cryptoPriv = priv.PrivateKey
	default:
		// We return a not implemented error because agent.AddedKey only
		// supports plain RSA, ECDSA, and ED25519 keys. Non-standard private
		// keys, like hardware-based private keys, will require custom solutions
		// which may not be included in their initial implementation. This will
		// only affect functionality related to agent forwarding, so we give the
		// caller the ability to handle the error gracefully.
		return agent.AddedKey{}, trace.NotImplemented("cannot create an agent key using private key of type %T", priv)
	}

	// put a teleport identifier along with the teleport user into the comment field
	comment := agentKeyComment{user: sshCert.KeyId}
	return agent.AddedKey{
		PrivateKey:       cryptoPriv,
		Certificate:      sshCert,
		Comment:          comment.String(),
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}, nil
}
