/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAllowedDomain(t *testing.T) {
	t.Parallel()
	allowedDomains := []string{
		"example.com",
		"alt.example.com",
	}
	tests := []struct {
		name   string
		url    string
		assert require.BoolAssertionFunc
	}{
		{
			name:   "domain match",
			url:    "example.com",
			assert: require.True,
		},
		{
			name:   "subdomain match",
			url:    "somewhere.example.com",
			assert: require.True,
		},
		{
			name:   "no match",
			url:    "fake.example2.com",
			assert: require.False,
		},
		{
			name:   "empty",
			url:    "",
			assert: require.False,
		},
		{
			name:   "not a url",
			url:    "$$$$",
			assert: require.False,
		},
		{
			name:   "suffix matches, but not as subdomain",
			url:    "fakeexample.com",
			assert: require.False,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, isAllowedDomain(tc.url, allowedDomains))
		})
	}
}
