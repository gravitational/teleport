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

	"github.com/stretchr/testify/require"
)

func TestEqualFingerprints(t *testing.T) {
	tests := []struct {
		name  string
		a     string
		b     string
		check require.BoolAssertionFunc
	}{
		{
			name:  "equal",
			a:     "SHA256:fingerprint",
			b:     "SHA256:fingerprint",
			check: require.True,
		},
		{
			name:  "not equal",
			a:     "SHA256:fingerprint",
			b:     "SHA256:fingerprint2",
			check: require.False,
		},
		{
			name:  "equal without prefix",
			a:     "SHA256:fingerprint",
			b:     "fingerprint",
			check: require.True,
		},
		{
			name:  "equal fold",
			a:     "FINGERPRINT",
			b:     "SHA256:fingerprint",
			check: require.True,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(t, EqualFingerprints(test.a, test.b))
		})
	}
}
