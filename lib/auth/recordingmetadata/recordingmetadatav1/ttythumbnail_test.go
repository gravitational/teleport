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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestTtyThumbnailGenerator_HandleEvent(t *testing.T) {
	startTime := time.Now()

	t.Run("session start sets terminal size", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(sessionStartEvent(startTime, "100:50")))

		thumb := gen.produceThumbnail()

		require.Equal(t, int32(100), thumb.Cols)
		require.Equal(t, int32(50), thumb.Rows)
	})

	t.Run("resize updates terminal size", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(sessionStartEvent(startTime, "80:24")))
		require.NoError(t, gen.handleEvent(resizeEvent(startTime.Add(1*time.Second), "120:40")))

		thumb := gen.produceThumbnail()

		require.Equal(t, int32(120), thumb.Cols)
		require.Equal(t, int32(40), thumb.Rows)
	})

	t.Run("session print writes data to terminal", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(sessionStartEvent(startTime, "80:24")))
		require.NoError(t, gen.handleEvent(sessionPrintEvent(startTime.Add(1*time.Second), "Hello World\r\n")))

		thumb := gen.produceThumbnail()

		require.NotEmpty(t, thumb.Svg)
		require.Contains(t, string(thumb.Svg), "Hello")
		require.Contains(t, string(thumb.Svg), "World")
	})

	t.Run("unhandled event types are ignored", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(&apievents.SessionEnd{
			Metadata: apievents.Metadata{Type: "session.end", Time: startTime},
		}))
		require.NoError(t, gen.handleEvent(&apievents.SessionJoin{
			Metadata: apievents.Metadata{Type: "session.join", Time: startTime},
		}))
	})

	t.Run("malformed terminal size returns error on start", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()
		err := gen.handleEvent(sessionStartEvent(startTime, "invalid"))

		require.Error(t, err)
		require.Contains(t, err.Error(), "parsing terminal size")
	})

	t.Run("malformed terminal size returns error on resize", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()
		err := gen.handleEvent(resizeEvent(startTime, "not:valid:size"))

		require.Error(t, err)
		require.Contains(t, err.Error(), "parsing terminal size")
	})
}

func TestTtyThumbnailGenerator_ProduceThumbnail(t *testing.T) {
	startTime := time.Now()

	t.Run("produces valid thumbnail with all fields", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(sessionStartEvent(startTime, "80:24")))
		require.NoError(t, gen.handleEvent(sessionPrintEvent(startTime.Add(1*time.Second), "hello\r\n")))

		thumb := gen.produceThumbnail()

		require.NotNil(t, thumb)
		require.Equal(t, int32(80), thumb.Cols)
		require.Equal(t, int32(24), thumb.Rows)
		require.NotEmpty(t, thumb.Svg)
		require.Contains(t, string(thumb.Svg), "<svg")
		require.Contains(t, string(thumb.Svg), "hello")
	})

	t.Run("cursor tracks print output position", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(sessionStartEvent(startTime, "80:24")))

		thumb0 := gen.produceThumbnail()
		require.Equal(t, int32(0), thumb0.CursorX)
		require.Equal(t, int32(0), thumb0.CursorY)

		require.NoError(t, gen.handleEvent(sessionPrintEvent(startTime.Add(1*time.Second), "line1\r\nline2\r\nline3\r\n")))

		thumb := gen.produceThumbnail()
		require.Equal(t, int32(0), thumb.CursorX, "cursor should be at start of line after \\r\\n")
		require.Equal(t, int32(3), thumb.CursorY, "cursor should be on 4th row after 3 lines")
	})

	t.Run("reflects latest terminal size after resize", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(sessionStartEvent(startTime, "80:24")))
		require.NoError(t, gen.handleEvent(sessionPrintEvent(startTime.Add(1*time.Second), "before resize\r\n")))
		require.NoError(t, gen.handleEvent(resizeEvent(startTime.Add(2*time.Second), "200:50")))

		thumb := gen.produceThumbnail()

		require.Equal(t, int32(200), thumb.Cols)
		require.Equal(t, int32(50), thumb.Rows)
		require.Contains(t, string(thumb.Svg), "before")
		require.Contains(t, string(thumb.Svg), "resize")
	})

	t.Run("snapshots are independent after more output", func(t *testing.T) {
		gen := newTTYThumbnailGenerator()

		require.NoError(t, gen.handleEvent(sessionStartEvent(startTime, "80:24")))
		require.NoError(t, gen.handleEvent(sessionPrintEvent(startTime.Add(1*time.Second), "first\r\n")))

		thumb1 := gen.produceThumbnail()

		require.NoError(t, gen.handleEvent(sessionPrintEvent(startTime.Add(2*time.Second), "second\r\n")))

		thumb2 := gen.produceThumbnail()

		require.NotEqual(t, thumb1.Svg, thumb2.Svg, "thumbnails should differ after more output")
		require.NotContains(t, string(thumb1.Svg), "second", "first snapshot should not contain later output")
		require.Contains(t, string(thumb2.Svg), "first", "second snapshot should still contain earlier output")
		require.Contains(t, string(thumb2.Svg), "second")
		require.Greater(t, thumb2.CursorY, thumb1.CursorY, "cursor should have advanced")
	})
}
