/*
Copyright 2015-2017 Gravitational, Inc.

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
	"crypto"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/trace"
)

// NewSigner returns new ssh Signer from private key + certificate pair.  The
// signer can be used to create "auth methods" i.e. login into Teleport SSH
// servers.
func NewSigner(keyBytes, certBytes []byte) (ssh.Signer, error) {
	keySigner, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse SSH private key")
	}

	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse SSH certificate")
	}

	cert, ok := pubkey.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("expected SSH certificate, got %T ", pubkey)
	}

	return ssh.NewCertSigner(cert, keySigner)
}

// CryptoPublicKey extracts public key from RSA public key in authorized_keys format
func CryptoPublicKey(publicKey []byte) (crypto.PublicKey, error) {
	// reuse the same RSA keys for SSH and TLS keys
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cryptoPubKey, ok := pubKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, trace.BadParameter("expected ssh.CryptoPublicKey, got %T", pubKey)
	}
	return cryptoPubKey.CryptoPublicKey(), nil
}
