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

package joinuri_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/config/joinuri"
)

func TestParseJoinURI(t *testing.T) {
	tests := []struct {
		uri         string
		expect      *joinuri.JoinURI
		expectError require.ErrorAssertionFunc
	}{
		{
			uri: "tbot+proxy+token://asdf@example.com:1234",
			expect: &joinuri.JoinURI{
				AddressKind:         connection.AddressKindProxy,
				Token:               "asdf",
				JoinMethod:          types.JoinMethodToken,
				Address:             "example.com:1234",
				JoinMethodParameter: "",
			},
		},
		{
			uri: "tbot+auth+bound-keypair://token:param@example.com",
			expect: &joinuri.JoinURI{
				AddressKind:         connection.AddressKindAuth,
				Token:               "token",
				JoinMethod:          types.JoinMethodBoundKeypair,
				Address:             "example.com",
				JoinMethodParameter: "param",
			},
		},
		{
			uri: "",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "unsupported joining URI scheme")
			},
		},
		{
			uri: "tbot+foo+token://example.com",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "address kind must be one of")
			},
		},
		{
			uri: "tbot+proxy+bar://example.com",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "unsupported join method")
			},
		},
		{
			uri: "https://example.com",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "unsupported joining URI scheme")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			parsed, err := joinuri.Parse(tt.uri)
			if tt.expectError == nil {
				require.NoError(t, err)
			} else {
				tt.expectError(t, err)
			}

			require.Empty(t, cmp.Diff(parsed, tt.expect))
		})
	}
}
