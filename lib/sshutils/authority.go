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
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// GetCheckers returns public keys that can be used to check cert authorities
func GetCheckers(ca types.CertAuthority) ([]ssh.PublicKey, error) {
	keys := ca.GetTrustedSSHKeyPairs()
	out := make([]ssh.PublicKey, 0, len(keys))
	for _, kp := range keys {
		key, _, _, _, err := ssh.ParseAuthorizedKey(kp.PublicKey)
		if err != nil {
			return nil, trace.BadParameter("invalid authority public key (len=%d): %v", len(kp.PublicKey), err)
		}
		out = append(out, key)
	}
	return out, nil
}

// GetSigners returns SSH signers for the provided authority.
func GetSigners(ca types.CertAuthority) ([]ssh.Signer, error) {
	var signers []ssh.Signer
	for _, kp := range ca.GetActiveKeys().SSH {
		if len(kp.PrivateKey) == 0 {
			continue
		}
		signer, err := ssh.ParsePrivateKey(kp.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

// ValidateSigners returns a list of signers that could be used to sign keys.
func ValidateSigners(ca types.CertAuthority) error {
	keys := ca.GetActiveKeys().SSH
	for _, kp := range keys {
		// PrivateKeys may be missing when loaded for use outside of the auth
		// server.
		if len(kp.PrivateKey) == 0 {
			continue
		}
		// TODO(nic): validate PKCS11 signers
		if kp.PrivateKeyType == types.PrivateKeyType_RAW {
			if _, err := ssh.ParsePrivateKey(kp.PrivateKey); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}
