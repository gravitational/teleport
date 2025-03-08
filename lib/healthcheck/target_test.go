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

package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		expectError bool
	}{
		{
			name:        "http without port",
			input:       "http://example.com",
			want:        "example.com:80",
			expectError: false,
		},
		{
			name:        "https without port",
			input:       "https://example.com",
			want:        "example.com:443",
			expectError: false,
		},
		{
			name:        "http with explicit port",
			input:       "http://example.com:8080",
			want:        "example.com:8080",
			expectError: false,
		},
		{
			name:        "https with explicit port",
			input:       "https://example.com:8443",
			want:        "example.com:8443",
			expectError: false,
		},
		{
			name:        "address with explicit port and no scheme",
			input:       "example.com:1234",
			want:        "example.com:1234",
			expectError: false,
		},
		{
			name:        "address without scheme and port",
			input:       "example.com",
			want:        "example.com:443",
			expectError: false,
		},
		{
			name:        "malformed URL with scheme 1",
			input:       "http:example.com",
			want:        "",
			expectError: true,
		},
		{
			name:        "malformed URL with scheme 2",
			input:       "http:example.com:80",
			want:        "",
			expectError: true,
		},
		{
			name:        "malformed URL with scheme 3",
			input:       "http://",
			want:        "",
			expectError: true,
		},
		{
			name:        "empty input",
			input:       "",
			want:        "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeAddress(tc.input)
			if tc.expectError {
				require.Empty(t, got)
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
