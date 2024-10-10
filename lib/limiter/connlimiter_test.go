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

package limiter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/limiter"
)

func TestConnectionsLimiter(t *testing.T) {
	l := limiter.NewConnectionsLimiter(0)

	for i := 0; i < 10; i++ {
		require.NoError(t, l.AcquireConnection("token1"))
	}
	for i := 0; i < 5; i++ {
		require.NoError(t, l.AcquireConnection("token2"))
	}

	for i := 0; i < 10; i++ {
		l.ReleaseConnection("token1")
	}
	for i := 0; i < 5; i++ {
		l.ReleaseConnection("token2")
	}

	l = limiter.NewConnectionsLimiter(5)

	for i := 0; i < 5; i++ {
		require.NoError(t, l.AcquireConnection("token1"))
	}

	for i := 0; i < 5; i++ {
		require.NoError(t, l.AcquireConnection("token2"))
	}
	for i := 0; i < 5; i++ {
		require.Error(t, l.AcquireConnection("token2"))
	}

	for i := 0; i < 10; i++ {
		l.ReleaseConnection("token1")
		require.NoError(t, l.AcquireConnection("token1"))
	}

	for i := 0; i < 5; i++ {
		l.ReleaseConnection("token2")
	}
	for i := 0; i < 5; i++ {
		require.NoError(t, l.AcquireConnection("token2"))
	}
}
