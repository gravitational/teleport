/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sshutils

import (
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
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
