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
	"image"
	"image/png"
	"math"

	"github.com/gravitational/trace"
	"golang.org/x/image/draw"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate"
)

const (
	// thumbnailMaxDimensions is the maximum dimension (width or height) for desktop thumbnails. The other dimension is
	// computed from the screen's actual aspect ratio to avoid black bars.
	thumbnailMaxDimensions = 1536

	// cursorVisibleZoom is the zoom level applied when the cursor is visible
	cursorVisibleZoom = 2.0
)

type desktopThumbnailGenerator struct {
	rdpstate *rdpstate.RDPState
}

func newDesktopThumbnailGenerator() *desktopThumbnailGenerator {
	return &desktopThumbnailGenerator{
		rdpstate: rdpstate.New(),
	}
}

func (d *desktopThumbnailGenerator) handleEvent(evt apievents.AuditEvent) error {
	switch e := evt.(type) {
	case *apievents.DesktopRecording:
		return d.handleDesktopRecording(e)
	}

	return nil
}

func (d *desktopThumbnailGenerator) handleDesktopRecording(evt *apievents.DesktopRecording) error {
	return d.rdpstate.HandleMessage(evt)
}

func (d *desktopThumbnailGenerator) produceThumbnail() (*pb.SessionRecordingThumbnail, error) {
	img := d.rdpstate.Image()
	if img == nil {
		return nil, trace.BadParameter("rdp state has no image")
	}

	cursor := d.rdpstate.CursorState()

	bounds := img.Bounds()

	screenWidth := bounds.Dx()
	screenHeight := bounds.Dy()

	if cursor.Visible {
		bounds = calculateCropBounds(bounds, cursor)
	}

	cropW := bounds.Dx()
	cropH := bounds.Dy()

	// Scale the crop to fit within thumbnailMaxDimensions.
	thumbW, thumbH := cropW, cropH
	if thumbW > thumbH {
		if thumbW > thumbnailMaxDimensions {
			thumbH = thumbH * thumbnailMaxDimensions / thumbW
			thumbW = thumbnailMaxDimensions
		}
	} else {
		if thumbH > thumbnailMaxDimensions {
			thumbW = thumbW * thumbnailMaxDimensions / thumbH
			thumbH = thumbnailMaxDimensions
		}
	}

	thumbImg := image.NewRGBA(image.Rect(0, 0, thumbW, thumbH))
	draw.CatmullRom.Scale(thumbImg, thumbImg.Bounds(), img, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, thumbImg); err != nil {
		return nil, trace.Wrap(err, "encoding thumbnail PNG")
	}

	return &pb.SessionRecordingThumbnail{
		CursorX:       int32(cursor.X),
		CursorY:       int32(cursor.Y),
		CursorVisible: cursor.Visible,
		ScreenWidth:   int32(screenWidth),
		ScreenHeight:  int32(screenHeight),
		Png:           buf.Bytes(),
	}, nil
}

func (d *desktopThumbnailGenerator) release() {
	d.rdpstate.Release()
}

func calculateCropBounds(bounds image.Rectangle, cursor decoder.CursorState) image.Rectangle {
	screenW := float64(bounds.Dx())
	screenH := float64(bounds.Dy())

	visibleW := screenW / cursorVisibleZoom
	visibleH := screenH / cursorVisibleZoom

	// Center on cursor, clamped so the crop stays within the image.
	cropX := math.Max(0, math.Min(screenW-visibleW, float64(cursor.X)-visibleW/2))
	cropY := math.Max(0, math.Min(screenH-visibleH, float64(cursor.Y)-visibleH/2))

	return image.Rect(
		int(cropX), int(cropY),
		int(cropX+visibleW), int(cropY+visibleH),
	)
}
