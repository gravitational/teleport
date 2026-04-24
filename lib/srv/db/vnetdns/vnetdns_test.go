// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnetdns

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	t.Parallel()

	const validChars = "0123456789abcdefghijklmnopqrstuv"
	// 8 bytes encoded as base32hex without padding = ceil(64/5) = 13 chars.
	const expectedLen = 13

	tests := []string{
		"db",
		"db-a",
		"my-database",
		"DB",
		"db with spaces",
		strings.Repeat("a", 256),
	}

	seen := make(map[string]string)
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			got := Name(name)
			require.Len(t, got, expectedLen)
			for _, c := range got {
				require.Contains(t, validChars, string(c),
					"output %q contains invalid character %q", got, c)
			}
			require.Equal(t, got, Name(name), "function must be deterministic")

			if other, ok := seen[got]; ok {
				t.Fatalf("collision: %q and %q both hash to %q", name, other, got)
			}
			seen[got] = name
		})
	}
}
