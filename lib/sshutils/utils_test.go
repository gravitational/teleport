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
package sshutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinHostPort(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "example.com:1234", JoinHostPort("example.com", 1234))
}

func TestSplitHostPort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		addr       string
		expectHost string
		expectPort uint32
		assertErr  assert.ErrorAssertionFunc
	}{
		{
			name:      "empty",
			assertErr: assert.Error,
		},
		{
			name:       "full host and port",
			addr:       "example.com:1234",
			expectHost: "example.com",
			expectPort: 1234,
			assertErr:  assert.NoError,
		},
		{
			name:       "without port",
			addr:       "example.com",
			expectHost: "example.com",
			assertErr:  assert.NoError,
		},
		{
			name:       "ipv6 addr",
			addr:       "[::1]:80",
			expectHost: "::1",
			expectPort: 80,
			assertErr:  assert.NoError,
		},
		{
			name:       "ipv6 addr without port",
			addr:       "[::1]",
			expectHost: "::1",
			assertErr:  assert.NoError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := SplitHostPort(tc.addr)
			tc.assertErr(t, err)
			assert.Equal(t, tc.expectHost, host)
			assert.Equal(t, tc.expectPort, port)
		})
	}
}
