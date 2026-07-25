//go:build desktop_access_rdp || rust_rdp_decoder

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
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
)

func TestDesktopThumbnailGenerator_ThumbnailScaling(t *testing.T) {
	startTime := time.Now()

	const maxDim = 64

	tests := []struct {
		name           string
		screenW        uint32
		screenH        uint32
		expectedThumbW int
		expectedThumbH int
	}{
		{
			name:           "small screen, no scaling needed",
			screenW:        32,
			screenH:        24,
			expectedThumbW: 32,
			expectedThumbH: 24,
		},
		{
			name:           "landscape, width exceeds max and is scaled down",
			screenW:        128,
			screenH:        72,
			expectedThumbW: 64,
			expectedThumbH: 36,
		},
		{
			name:           "portrait, height exceeds max and is scaled down",
			screenW:        72,
			screenH:        128,
			expectedThumbW: 36,
			expectedThumbH: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := newDesktopThumbnailGenerator()
			defer gen.release()

			require.NoError(t, gen.handleEvent(desktopServerHelloEvent(t, startTime, tt.screenW, tt.screenH)))
			require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(1*time.Second), rdpstatetest.BuildBitmapPDU(0, 0, 4, 2, rdpstatetest.RGB565White))))

			thumb, err := gen.produceThumbnail(maxDim)
			require.NoError(t, err)
			require.NotNil(t, thumb)

			require.NotEmpty(t, thumb.Png)
			img, err := png.Decode(bytes.NewReader(thumb.Png))
			require.NoError(t, err)
			require.Equal(t, tt.expectedThumbW, img.Bounds().Dx())
			require.Equal(t, tt.expectedThumbH, img.Bounds().Dy())

			require.Equal(t, int32(tt.screenW), thumb.ScreenWidth)
			require.Equal(t, int32(tt.screenH), thumb.ScreenHeight)
		})
	}
}

func TestDesktopThumbnailGenerator_CursorMetadata(t *testing.T) {
	startTime := time.Now()

	tests := []struct {
		name            string
		pointerPDUs     [][]byte
		expectedVisible bool
		expectedX       int32
		expectedY       int32
	}{
		{
			name:            "hidden by default",
			expectedVisible: false,
		},
		{
			name: "position is captured",
			pointerPDUs: [][]byte{
				rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed),
				rdpstatetest.BuildPointerPositionPDU(50, 60),
			},
			expectedVisible: true,
			expectedX:       50,
			expectedY:       60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := newDesktopThumbnailGenerator()
			defer gen.release()

			require.NoError(t, gen.handleEvent(desktopServerHelloEvent(t, startTime, 100, 100)))

			for i, pdu := range tt.pointerPDUs {
				require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(time.Duration(i+1)*time.Second), pdu)))
			}

			thumb, err := gen.produceThumbnail(thumbnailMaxDimensions)
			require.NoError(t, err)

			require.Equal(t, tt.expectedVisible, thumb.CursorVisible)
			require.Equal(t, tt.expectedX, thumb.CursorX)
			require.Equal(t, tt.expectedY, thumb.CursorY)
		})
	}
}

func TestDesktopThumbnailGenerator_ProduceThumbnail(t *testing.T) {
	startTime := time.Now()

	t.Run("no image returns error", func(t *testing.T) {
		gen := newDesktopThumbnailGenerator()
		defer gen.release()

		_, err := gen.produceThumbnail(thumbnailMaxDimensions)
		require.Error(t, err)
	})

	t.Run("snapshots are independent after more updates", func(t *testing.T) {
		gen := newDesktopThumbnailGenerator()
		defer gen.release()

		require.NoError(t, gen.handleEvent(desktopServerHelloEvent(t, startTime, 100, 100)))
		require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(1*time.Second), rdpstatetest.BuildBitmapPDU(0, 0, 4, 2, rdpstatetest.RGB565White))))

		thumb1, err := gen.produceThumbnail(thumbnailMaxDimensions)
		require.NoError(t, err)

		require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(2*time.Second), rdpstatetest.BuildBitmapPDU(10, 10, 4, 2, rdpstatetest.RGB565Red))))

		thumb2, err := gen.produceThumbnail(thumbnailMaxDimensions)
		require.NoError(t, err)

		require.NotEqual(t, thumb1.Png, thumb2.Png, "thumbnails should differ after more updates")
	})

	t.Run("cursor visible triggers crop zoom", func(t *testing.T) {
		gen := newDesktopThumbnailGenerator()
		defer gen.release()

		require.NoError(t, gen.handleEvent(desktopServerHelloEvent(t, startTime, 64, 48)))
		require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(1*time.Second), rdpstatetest.BuildBitmapPDU(0, 0, 4, 2, rdpstatetest.RGB565White))))

		thumbNoCursor, err := gen.produceThumbnail(thumbnailMaxDimensions)
		require.NoError(t, err)

		// Make cursor visible at center of screen.
		require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(2*time.Second), rdpstatetest.BuildNewPointerPDU(2, 2, 0, 0, rdpstatetest.BGRARed))))
		require.NoError(t, gen.handleEvent(desktopFastPathEvent(t, startTime.Add(3*time.Second), rdpstatetest.BuildPointerPositionPDU(32, 24))))

		thumbWithCursor, err := gen.produceThumbnail(thumbnailMaxDimensions)
		require.NoError(t, err)

		require.NotEqual(t, thumbNoCursor.Png, thumbWithCursor.Png)
		require.True(t, thumbWithCursor.CursorVisible)
	})
}

func desktopServerHelloEvent(t *testing.T, eventTime time.Time, width, height uint32) *apievents.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.EncodeTDPBServerHello(width, height)
	require.NoError(t, err)

	evt.Metadata = apievents.Metadata{Type: events.DesktopRecordingEvent, Time: eventTime}

	return evt
}

func desktopFastPathEvent(t *testing.T, eventTime time.Time, pdu []byte) *apievents.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
	require.NoError(t, err)

	evt.Metadata = apievents.Metadata{Type: events.DesktopRecordingEvent, Time: eventTime}

	return evt
}
