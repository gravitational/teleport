//go:build unix

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

package socketpair

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocketserverBasics(t *testing.T) {
	left, right, err := NewFDs()
	require.NoError(t, err)

	listener, err := ListenerFromFD(left)
	require.NoError(t, err)

	dialer, err := DialerFromFD(right)
	require.NoError(t, err)

	go func() {
		c, err := dialer.Dial()
		if !assert.NoError(t, err) {
			return
		}

		_, err = c.Write([]byte("hello"))
		assert.NoError(t, err)

		assert.NoError(t, c.Close())
	}()

	c, err := listener.Accept()
	require.NoError(t, err)

	b, err := io.ReadAll(c)
	require.NoError(t, err)

	require.Equal(t, []byte("hello"), b)
}
