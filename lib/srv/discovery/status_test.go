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

package discovery

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

func TestTruncateErrorMessage(t *testing.T) {
	for _, tt := range []struct {
		name     string
		in       discoveryconfig.Status
		expected *string
	}{
		{
			name:     "nil error message",
			in:       discoveryconfig.Status{},
			expected: nil,
		},
		{
			name:     "small error messages are not changed",
			in:       discoveryconfig.Status{ErrorMessage: stringPointer("small error message")},
			expected: stringPointer("small error message"),
		},
		{
			name:     "large error messages are truncated",
			in:       discoveryconfig.Status{ErrorMessage: stringPointer(strings.Repeat("A", 1024*100+1))},
			expected: stringPointer(strings.Repeat("A", 1024*100)),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateErrorMessage(tt.in)
			require.Equal(t, tt.expected, got)
		})
	}
}
