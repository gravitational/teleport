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

package restrictedsession

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseIPSpec(f *testing.F) {
	f.Add("127.0.0.111")
	f.Add("127.0.0.111/8")
	f.Add("192.168.0.0/16")
	f.Add("2001:0db8:85a3:0000:0000:8a2e:0370:7334")
	f.Add("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64")
	f.Add("2001:db8::ff00:42:8329")
	f.Add("2001:db8::ff00:42:8329/48")

	f.Fuzz(func(t *testing.T, cidr string) {
		require.NotPanics(t, func() {
			ParseIPSpec(cidr)
		})
	})
}
