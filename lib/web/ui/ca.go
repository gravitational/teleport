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

type SSHKey struct {
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
}

type TLSKey struct {
	Cert string `json:"cert"`
}

type JWTKey struct {
	PublicKey string `json:"publicKey"`
}

type CAKeySet struct {
	SSH []SSHKey `json:"ssh"`
	TLS []TLSKey `json:"tls"`
	JWT []JWTKey `json:"jwt"`
}

// MakeCAKeySet creates a CAKeySet object for the web ui. SSH key fingerprints
// are exported but private keys are excluded.
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
