/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package token

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
)

func TestVerify(t *testing.T) {
	tests := []struct {
		name      string
		token     []byte
		assertErr func(t require.TestingT, err error, msgAndArgs ...any)
	}{
		{
			name:  "token too short",
			token: []byte("abc123"),
			assertErr: func(t require.TestingT, err error, msgAndArgs ...any) {
				require.ErrorContains(t, err, "token is too short")
			},
		},
		{
			name:  "token too long",
			token: make([]byte, defaults.MaxTokenLength+1),
			assertErr: func(t require.TestingT, err error, msgAndArgs ...any) {
				require.ErrorContains(t, err, "token is too long")
			},
		},
		{
			name:  "token doesn't have enough entropy",
			token: bytes.Repeat([]byte("A"), defaults.MinTokenLength),
			assertErr: func(t require.TestingT, err error, msgAndArgs ...any) {
				require.ErrorContains(t, err, "token is not strong enough")
			},
		},
		{
			name:      "token is good",
			token:     []byte("noonewilleverguessthistoken!!!!!"),
			assertErr: require.NoError,
		},
		{
			name:      "basic token is good",
			token:     []byte("1234567890abcdefghijklmnopqrstuv"),
			assertErr: require.NoError,
		},
		{
			name:      "random token is good",
			token:     []byte("b0fbcc4bee3b8a6523af6941869642e0"),
			assertErr: require.NoError,
		},
		{
			name:      "long random token is good",
			token:     []byte("9a030be95f0dc7d70d8fb549e55074fa453de9c21690eb4a66f563c925c52766"),
			assertErr: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Verify(tt.token)
			tt.assertErr(t, err)
		})
	}
}
