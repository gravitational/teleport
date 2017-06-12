/*
Copyright 2015 Gravitational, Inc.

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
	"fmt"

	"golang.org/x/crypto/ssh"
)

// NewSigner returns new ssh Signer from private key + certificate pair.  The
// signer can be used to create "auth methods" i.e. login into Teleport SSH
// servers.
func NewSigner(keyBytes, certBytes []byte) (ssh.Signer, error) {
	keySigner, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host private key, err: %v", err)
	}

	pubkey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse server CA certificate '%v', err: %v",
			string(certBytes), err)
	}

	cert, ok := pubkey.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected CA certificate, got %T ", pubkey)
	}

	return ssh.NewCertSigner(cert, keySigner)
}
