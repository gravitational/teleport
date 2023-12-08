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

package connection

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseRedisAddress(f *testing.F) {
	f.Add("foo:1234")
	f.Add(URIScheme + "://foo")
	f.Add(URIScheme + "://foo:1234?mode=standalone")
	f.Add(URIScheme + "://foo:1234?mode=cluster")

	f.Fuzz(func(t *testing.T, addr string) {
		require.NotPanics(t, func() {
			ParseRedisAddress(addr)
		})
	})
}
