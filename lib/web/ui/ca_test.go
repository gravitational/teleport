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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/sshutils"
)

func TestMakeCAKeySet(t *testing.T) {
	sshFingerprint, err := sshutils.AuthorizedKeyFingerprint([]byte(fixtures.SSHCAPublicKey))
	require.NoError(t, err)

	tests := []struct {
		name       string
		input      *types.CAKeySet
		checkError require.ErrorAssertionFunc
		expect     *CAKeySet
	}{
		{
			name:       "empty",
			input:      &types.CAKeySet{},
			checkError: require.NoError,
			expect:     &CAKeySet{},
		},
		{
			name: "SSH keys",
			input: &types.CAKeySet{
				SSH: []*types.SSHKeyPair{{
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
				}},
			},
			checkError: require.NoError,
			expect: &CAKeySet{
				SSH: []SSHKey{{
					PublicKey:   fixtures.SSHCAPublicKey,
					Fingerprint: sshFingerprint,
				}},
			},
		},
		{
			name: "TLS keys",
			input: &types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: []byte(fixtures.TLSCACertPEM),
					Key:  []byte(fixtures.TLSCAKeyPEM),
				}},
			},
			checkError: require.NoError,
			expect: &CAKeySet{
				TLS: []TLSKey{{Cert: fixtures.TLSCACertPEM}},
			},
		},
		{
			name: "JWT keys",
			input: &types.CAKeySet{
				JWT: []*types.JWTKeyPair{{
					PublicKey:  []byte(fixtures.JWTSignerPublicKey),
					PrivateKey: []byte(fixtures.JWTSignerPrivateKey),
				}},
			},
			checkError: require.NoError,
			expect: &CAKeySet{
				JWT: []JWTKey{{PublicKey: fixtures.JWTSignerPublicKey}},
			},
		},
		{
			name: "multiple keys",
			input: &types.CAKeySet{
				SSH: []*types.SSHKeyPair{
					{PublicKey: []byte(fixtures.SSHCAPublicKey)},
					{PublicKey: []byte(fixtures.SSHCAPublicKey)},
				},
				TLS: []*types.TLSKeyPair{
					{Cert: []byte(fixtures.TLSCACertPEM)},
					{Cert: []byte(fixtures.TLSCACertPEM)},
				},
				JWT: []*types.JWTKeyPair{
					{PublicKey: []byte(fixtures.JWTSignerPublicKey)},
					{PublicKey: []byte(fixtures.JWTSignerPublicKey)},
				},
			},
			checkError: require.NoError,
			expect: &CAKeySet{
				SSH: []SSHKey{
					{PublicKey: fixtures.SSHCAPublicKey, Fingerprint: sshFingerprint},
					{PublicKey: fixtures.SSHCAPublicKey, Fingerprint: sshFingerprint},
				},
				TLS: []TLSKey{
					{Cert: fixtures.TLSCACertPEM},
					{Cert: fixtures.TLSCACertPEM},
				},
				JWT: []JWTKey{
					{PublicKey: fixtures.JWTSignerPublicKey},
					{PublicKey: fixtures.JWTSignerPublicKey},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := MakeCAKeySet(test.input)
			test.checkError(t, err)
			require.Equal(t, test.expect, actual)
		})
	}
}
