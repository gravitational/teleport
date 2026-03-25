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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceNamer(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		namer := newServiceNamer()

		// Chosen name is used.
		name, err := namer.pickName("type/1", "foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", name)

		// Next name in the "sequence" for that service type is used.
		t12, err := namer.pickName("type/1", "")
		require.NoError(t, err)
		assert.Equal(t, "type-1-2", t12)

		// First name in sequence for new service type is used.
		t21, err := namer.pickName("type/2", "")
		require.NoError(t, err)
		assert.Equal(t, "type-2-1", t21)
	})

	t.Run("reserved name", func(t *testing.T) {
		namer := newServiceNamer()

		_, err := namer.pickName("foo", "heartbeat")
		require.ErrorContains(t, err, `service name "heartbeat" is reserved for internal use`)
	})

	t.Run("invalid name", func(t *testing.T) {
		namer := newServiceNamer()

		_, err := namer.pickName("foo", "hello, world")
		require.ErrorContains(t, err, `invalid service name: "hello, world", may only contain lowercase letters, numbers, hyphens, underscores, or plus symbols`)
	})

	t.Run("named used twice", func(t *testing.T) {
		namer := newServiceNamer()

		_, err := namer.pickName("foo", "bar")
		require.NoError(t, err)

		_, err = namer.pickName("foo", "bar")
		require.ErrorContains(t, err, `service name "bar" used more than once`)
	})

	t.Run("chosen name conflicts with generated name", func(t *testing.T) {
		namer := newServiceNamer()

		_, err := namer.pickName("foo", "foo-2")
		require.NoError(t, err)

		_, err = namer.pickName("foo", "")
		require.ErrorContains(t, err, `service name "foo-2" conflicts with an automatically generated service name`)
	})
}
