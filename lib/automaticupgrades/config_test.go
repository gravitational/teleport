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

package automaticupgrades

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func TestIsEnabled(t *testing.T) {
	t.Run("no env var returns false", func(t *testing.T) {
		t.Setenv(automaticUpgradesEnvar, "")
		require.False(t, IsEnabled())
	})
	t.Run("truthy value returns true", func(t *testing.T) {
		t.Setenv(automaticUpgradesEnvar, "1")
		require.True(t, IsEnabled())

		t.Setenv(automaticUpgradesEnvar, "TRUE")
		require.True(t, IsEnabled())
	})

	t.Run("falsy value returns false", func(t *testing.T) {
		t.Setenv(automaticUpgradesEnvar, "0")
		require.False(t, IsEnabled())

		t.Setenv(automaticUpgradesEnvar, "FALSE")
		require.False(t, IsEnabled())
	})

	t.Run("invalid value returns false and logs a warning message", func(t *testing.T) {
		hook := test.NewGlobal()
		defer hook.Reset()

		t.Setenv(automaticUpgradesEnvar, "foo")
		require.False(t, IsEnabled())

		require.Len(t, hook.Entries, 1)
		require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
		require.Equal(t, `unexpected value for ENV:TELEPORT_AUTOMATIC_UPGRADES: strconv.ParseBool: parsing "foo": invalid syntax`, hook.LastEntry().Message)
	})
}
