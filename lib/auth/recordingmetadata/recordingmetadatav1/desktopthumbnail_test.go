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
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
)

func TestDesktopThumbnailGenerator_UnhandledEvents(t *testing.T) {
	startTime := time.Now()
	gen := newDesktopThumbnailGenerator()

	// Non-DesktopRecording events should be silently ignored.
	require.NoError(t, gen.handleEvent(&apievents.SessionEnd{
		Metadata: apievents.Metadata{Type: events.SessionEndEvent, Time: startTime},
	}))
	require.NoError(t, gen.handleEvent(&apievents.SessionJoin{
		Metadata: apievents.Metadata{Type: events.SessionJoinEvent, Time: startTime},
	}))
	require.NoError(t, gen.handleEvent(&apievents.SessionStart{
		Metadata: apievents.Metadata{Type: events.SessionStartEvent, Time: startTime},
	}))
}

func TestCalculateCropBounds(t *testing.T) {
	tests := []struct {
		name     string
		bounds   image.Rectangle
		cursor   decoder.CursorState
		expected image.Rectangle
	}{
		{
			name:     "cursor centered on screen",
			bounds:   image.Rect(0, 0, 1920, 1080),
			cursor:   decoder.CursorState{Visible: true, X: 960, Y: 540},
			expected: image.Rect(480, 270, 1440, 810), // 50% zoom centered on cursor
		},
		{
			name:     "cursor at top-left corner clamps to origin",
			bounds:   image.Rect(0, 0, 1920, 1080),
			cursor:   decoder.CursorState{Visible: true, X: 0, Y: 0},
			expected: image.Rect(0, 0, 960, 540), // zoom centered on cursor would go out of bounds, so it clamps to the top-left corner
		},
		{
			name:     "cursor at bottom-right corner clamps to edge",
			bounds:   image.Rect(0, 0, 1920, 1080),
			cursor:   decoder.CursorState{Visible: true, X: 1920, Y: 1080},
			expected: image.Rect(960, 540, 1920, 1080), // zoom centered on cursor would go out of bounds, so it clamps to the bottom-right corner
		},
		{
			name:     "cursor near left edge",
			bounds:   image.Rect(0, 0, 1920, 1080),
			cursor:   decoder.CursorState{Visible: true, X: 100, Y: 540},
			expected: image.Rect(0, 270, 960, 810), // zoom centered on cursor would go out of bounds on the left, so it clamps to the left edge
		},
		{
			name:     "cursor near right edge",
			bounds:   image.Rect(0, 0, 1920, 1080),
			cursor:   decoder.CursorState{Visible: true, X: 1820, Y: 540},
			expected: image.Rect(960, 270, 1920, 810), // zoom centered on cursor would go out of bounds on the right, so it clamps to the right edge
		},
		{
			name:     "small screen",
			bounds:   image.Rect(0, 0, 100, 100),
			cursor:   decoder.CursorState{Visible: true, X: 50, Y: 50},
			expected: image.Rect(25, 25, 75, 75), // 50% zoom centered on cursor, no clamping needed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateCropBounds(tt.bounds, tt.cursor)
			require.Equal(t, tt.expected, result)
		})
	}
}
