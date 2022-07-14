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

	"github.com/gravitational/teleport/api/utils/sshutils/ppk"
	"github.com/gravitational/teleport/lib/auth/native"

	"github.com/gravitational/trace"
)

type KeyPair interface {
	PrivateKey() crypto.PrivateKey
	Signer() crypto.Signer
	PrivateKeyPEM() []byte
	PublicKeyPEM() []byte
	PPKFile() ([]byte, error)
	TLSCertificate(certRaw []byte) (tls.Certificate, error)
	Equals(KeyPair) bool
}

type RSAKeyPair struct {
	rsaPrivateKey *rsa.PrivateKey
	privateKeyPEM []byte
	publicKeyPEM  []byte
}

func GenerateRSAKeyPair() (*RSAKeyPair, error) {
	priv, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO ideally we only use raw keys for reading/writing to disk,
	// so we wouldn't need to pre-parse and store the values here.
	privPEM, pubPEM, err := native.ParseRSAPrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RSAKeyPair{
		rsaPrivateKey: priv,
		privateKeyPEM: privPEM,
		publicKeyPEM:  pubPEM,
	}, nil
}

// Retuns a new RSAKeyPair from an existing PEM-encoded RSA key pair.
func NewRSAKeyPair(priv, pub []byte) *RSAKeyPair {
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
	return r.publicKeyPEM
}

// TODO: remove this, we should not need to expose raw private keys
// - may need to add a way for KeyPair to write a private key to disk though
func (r *RSAKeyPair) PrivateKeyPEM() []byte {
	return r.privateKeyPEM
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
