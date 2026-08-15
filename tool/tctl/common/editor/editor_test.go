/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package editor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommand(t *testing.T) {
	t.Run("defaults to vi when nothing is set", func(t *testing.T) {
		t.Setenv("TELEPORT_EDITOR", "")
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "")
		require.Equal(t, "vi", command())
	})

	t.Run("TELEPORT_EDITOR takes precedence", func(t *testing.T) {
		t.Setenv("TELEPORT_EDITOR", "tele-ed")
		t.Setenv("VISUAL", "visual-ed")
		t.Setenv("EDITOR", "editor-ed")
		require.Equal(t, "tele-ed", command())
	})

	t.Run("VISUAL beats EDITOR", func(t *testing.T) {
		t.Setenv("TELEPORT_EDITOR", "")
		t.Setenv("VISUAL", "visual-ed")
		t.Setenv("EDITOR", "editor-ed")
		require.Equal(t, "visual-ed", command())
	})

	t.Run("EDITOR is the last resort before vi", func(t *testing.T) {
		t.Setenv("TELEPORT_EDITOR", "")
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "editor-ed")
		require.Equal(t, "editor-ed", command())
	})
}
