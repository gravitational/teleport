/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package recordingmetadatav1

import (
	"image"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
)

func TestProperty_CalculateCropBounds_NonNegativeOrigin(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bounds := genScreenBounds(t)
		cursor := genCursor(t)
		result := calculateCropBounds(bounds, cursor)

		require.GreaterOrEqual(t, result.Min.X, 0, "result=%v bounds=%v cursor=%+v", result, bounds, cursor)
		require.GreaterOrEqual(t, result.Min.Y, 0, "result=%v bounds=%v cursor=%+v", result, bounds, cursor)
	})
}

func TestProperty_CalculateCropBounds_WithinScreen(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bounds := genScreenBounds(t)
		cursor := genCursor(t)
		result := calculateCropBounds(bounds, cursor)

		require.LessOrEqual(t, result.Max.X, bounds.Max.X, "result=%v bounds=%v cursor=%+v", result, bounds, cursor)
		require.LessOrEqual(t, result.Max.Y, bounds.Max.Y, "result=%v bounds=%v cursor=%+v", result, bounds, cursor)
	})
}

func TestProperty_CalculateCropBounds_AtMostHalfPlusOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bounds := genScreenBounds(t)
		cursor := genCursor(t)
		result := calculateCropBounds(bounds, cursor)

		require.LessOrEqual(t, result.Dx(), bounds.Dx()/2+1, "result.Dx=%d bounds.Dx=%d", result.Dx(), bounds.Dx())
		require.LessOrEqual(t, result.Dy(), bounds.Dy()/2+1, "result.Dy=%d bounds.Dy=%d", result.Dy(), bounds.Dy())
	})
}

func TestProperty_CalculateCropBounds_ContainsCursorWhenOnScreen(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Below 4px per axis the cursor-centered crop can be empty and exclude the cursor.
		w := rapid.IntRange(4, 8192).Draw(t, "screen_w")
		h := rapid.IntRange(4, 8192).Draw(t, "screen_h")

		bounds := image.Rect(0, 0, w, h)
		cursor := decoder.CursorState{
			Visible: rapid.Bool().Draw(t, "visible"),
			X:       uint16(rapid.IntRange(0, w-1).Draw(t, "cursor_x")),
			Y:       uint16(rapid.IntRange(0, h-1).Draw(t, "cursor_y")),
		}

		result := calculateCropBounds(bounds, cursor)

		require.True(t,
			int(cursor.X) >= result.Min.X && int(cursor.X) < result.Max.X,
			"cursor.X=%d not in result.X=[%d,%d) bounds=%v",
			cursor.X, result.Min.X, result.Max.X, bounds)
		require.True(t,
			int(cursor.Y) >= result.Min.Y && int(cursor.Y) < result.Max.Y,
			"cursor.Y=%d not in result.Y=[%d,%d) bounds=%v",
			cursor.Y, result.Min.Y, result.Max.Y, bounds)
	})
}

// genScreenBounds generates a screen rectangle anchored at (0,0). Matches the real call-site shape where bounds come
// from RDP framebuffer dimensions.
func genScreenBounds(t *rapid.T) image.Rectangle {
	t.Helper()

	w := rapid.OneOf(
		rapid.Just(0),
		rapid.Just(1),
		rapid.Just(2),
		rapid.IntRange(2, 8192),
	).Draw(t, "screen_w")
	h := rapid.OneOf(
		rapid.Just(0),
		rapid.Just(1),
		rapid.Just(2),
		rapid.IntRange(2, 8192),
	).Draw(t, "screen_h")

	return image.Rect(0, 0, w, h)
}

func genCursor(t *rapid.T) decoder.CursorState {
	t.Helper()

	return decoder.CursorState{
		Visible: rapid.Bool().Draw(t, "visible"),
		X:       uint16(rapid.IntRange(0, 65535).Draw(t, "cursor_x")),
		Y:       uint16(rapid.IntRange(0, 65535).Draw(t, "cursor_y")),
	}
}
