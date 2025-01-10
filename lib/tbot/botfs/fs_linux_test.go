//go:build linux
// +build linux

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

package botfs

import (
	"testing"

	"github.com/joshlf/go-acl"
	"github.com/stretchr/testify/require"
)

func TestCompareACL(t *testing.T) {
	// Note: no named users or groups since we can't depend on names.
	readersA := []*ACLSelector{
		{User: "123"},
		{Group: "456"},
	}
	readersB := []*ACLSelector{
		{Group: "123"},
		{User: "456"},
	}

	testA, err := aclFromReaders(readersA, false)
	require.NoError(t, err)

	testB, err := aclFromReaders(readersB, false)
	require.NoError(t, err)

	tests := []struct {
		name      string
		expected  acl.ACL
		candidate acl.ACL
		assert    func(t *testing.T, issues []string)
	}{
		{
			name:      "matching",
			expected:  testA,
			candidate: testA,
			assert: func(t *testing.T, issues []string) {
				require.Empty(t, issues)
			},
		},
		{
			name:      "empty candidate",
			expected:  testA,
			candidate: acl.ACL{},
			assert: func(t *testing.T, issues []string) {
				require.Len(t, issues, 6)
				require.ElementsMatch(t, issues, []string{
					"missing required entry: u::rw-",
					"missing required entry: g::---",
					"missing required entry: m::r--",
					"missing required entry: o::---",
					"missing required entry: u:123:r--",
					"missing required entry: g:456:r--",
				})
			},
		},
		{
			name:      "mismatched",
			expected:  testA,
			candidate: testB,
			assert: func(t *testing.T, issues []string) {
				require.Len(t, issues, 4)
				require.ElementsMatch(t, issues, []string{
					"missing required entry: u:123:r--",
					"missing required entry: g:456:r--",
					"unexpected entry: g:123:r--",
					"unexpected entry: u:456:r--",
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := CompareACL(tt.expected, tt.candidate)
			tt.assert(t, issues)
		})
	}
}
