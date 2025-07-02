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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestParseJoinURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		expect      *JoinURIParams
		expectError require.ErrorAssertionFunc
	}{
		{
			uri: "tbot+proxy+token://asdf@example.com:1234",
			expect: &JoinURIParams{
				AddressKind:         AddressKindProxy,
				Token:               "asdf",
				JoinMethod:          types.JoinMethodToken,
				Address:             "example.com:1234",
				JoinMethodParameter: "",
			},
		},
		{
			uri: "tbot+auth+bound-keypair://token:param@example.com",
			expect: &JoinURIParams{
				AddressKind:         AddressKindAuth,
				Token:               "token",
				JoinMethod:          types.JoinMethodBoundKeypair,
				Address:             "example.com",
				JoinMethodParameter: "param",
			},
		},
		{
			uri: "",
			expectError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "unsupported joining URI scheme")
			},
		},
		{
			uri: "tbot+foo+token://example.com",
			expectError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "address kind must be one of")
			},
		},
		{
			uri: "tbot+proxy+bar://example.com",
			expectError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "unsupported join method")
			},
		},
		{
			uri: "https://example.com",
			expectError: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "unsupported joining URI scheme")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			parsed, err := ParseJoinURI(tt.uri)
			if tt.expectError == nil {
				require.NoError(t, err)
			} else {
				tt.expectError(t, err)
			}

			require.Empty(t, cmp.Diff(parsed, tt.expect))
		})
	}
}
