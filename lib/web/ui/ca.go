/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package ui

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/sshutils"
)

// SSHKey represents a SSH key.
type SSHKey struct {
	// PublicKey is the SSH public key.
	PublicKey string `json:"publicKey"`
	// Fingerprint is the SHA256 fingerprint of the key.
	Fingerprint string `json:"fingerprint"`
}

// TLSKey represents a TLS key.
type TLSKey struct {
	// Cert is a PEM encoded TLS cert
	Cert string `json:"cert"`
}

// JWTKey represents a JWT key.
type JWTKey struct {
	// PublicKey is public key.
	PublicKey string `json:"publicKey"`
}

// CAKeySet is the web app representation of types.CAKeySet which describes a
// set of CA keys owned by a certificate authority type.
type CAKeySet struct {
	// SSH contains SSH CA keys.
	SSH []SSHKey `json:"ssh"`
	// TLS contains TLS CA keys.
	TLS []TLSKey `json:"tls"`
	// JWT contains JWT CA keys.
	JWT []JWTKey `json:"jwt"`
}

// MakeCAKeySet creates a CAKeySet object for the web ui. SSH key fingerprints
// are exported alongside of SSH public keys. All privates are excluded.
func MakeCAKeySet(cas *types.CAKeySet) (*CAKeySet, error) {
	ret := CAKeySet{}
	for _, ssh := range cas.SSH {
		fingerprint, err := sshutils.AuthorizedKeyFingerprint(ssh.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ret.SSH = append(ret.SSH, SSHKey{
			PublicKey:   strings.TrimSpace(string(ssh.PublicKey)),
			Fingerprint: fingerprint,
		})
	}
	for _, tls := range cas.TLS {
		ret.TLS = append(ret.TLS, TLSKey{
			Cert: string(tls.Cert),
		})
	}
	for _, jwt := range cas.JWT {
		ret.JWT = append(ret.JWT, JWTKey{
			PublicKey: strings.TrimSpace(string(jwt.PublicKey)),
		})
	}
	return &ret, nil
}
