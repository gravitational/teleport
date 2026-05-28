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

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate"
)

const (
	// thumbnailMaxDimensions is the maximum dimension (width or height) for the representative
	// session thumbnail. The other dimension is computed from the screen's actual aspect ratio
	// to avoid black bars.
	thumbnailMaxDimensions = 1536

	// frameMaxDimensions is the maximum dimension for streamed timeline frames. Frames are
	// rendered smaller than the representative thumbnail since they're displayed at low
	// resolution in the scrubber UI, which keeps the metadata file size reasonable.
	frameMaxDimensions = 1024

	// cursorVisibleZoom is the zoom level applied when the cursor is visible
	cursorVisibleZoom = 2.0
)

type desktopThumbnailGenerator struct {
	rdpstate   *rdpstate.RDPState
	pngEncoder png.Encoder
	buf        bytes.Buffer
	// disabled is set when the RDP decoder is not available (e.g. nop builds without desktop_access_rdp). Once set, all
	// subsequent events are skipped so that metadata processing degrades to a no-op instead of failing.
	disabled bool
}

func newDesktopThumbnailGenerator() *desktopThumbnailGenerator {
	return &desktopThumbnailGenerator{
		rdpstate:   rdpstate.New(),
		pngEncoder: png.Encoder{CompressionLevel: png.BestSpeed},
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
	if d.disabled {
		return nil
	}

	err := d.rdpstate.HandleMessage(evt)
	if trace.IsNotImplemented(err) {
		d.disabled = true

		return nil
	}

	return err
}

// produceThumbnail generates a thumbnail from the current RDP state. If the cursor is visible, the thumbnail is zoomed
// to the area around the cursor. maxDim caps the longer side (in pixels) of the encoded PNG.
// NOTE: If the decoder is not available (e.g. in nop builds without desktop_access_rdp), this will return nil without
// error and all subsequent calls will be no-ops.
func (d *desktopThumbnailGenerator) produceThumbnail(maxDim int) (*pb.SessionRecordingThumbnail, error) {
	if d.disabled {
		return nil, nil
	}

	screenW, screenH := d.rdpstate.Dimensions()
	if screenW == 0 || screenH == 0 {
		return nil, trace.BadParameter("rdp state has no image")
	}

	cursor := d.rdpstate.CursorState()

	bounds := image.Rect(0, 0, int(screenW), int(screenH))
	if cursor.Visible {
		bounds = calculateCropBounds(bounds, cursor)
	}

	cropW := bounds.Dx()
	cropH := bounds.Dy()

	// Scale the crop to fit within maxDim.
	thumbW, thumbH := cropW, cropH
	if thumbW > thumbH {
		if thumbW > maxDim {
			thumbH = thumbH * maxDim / thumbW
			thumbW = maxDim
		}
	} else {
		if thumbH > maxDim {
			thumbW = thumbW * maxDim / thumbH
			thumbH = maxDim
		}
	}

	// The integer division above can floor a dimension to 0 for extreme aspect
	// ratios; clamp to at least 1px so we still produce a valid thumbnail.
	thumbW = max(thumbW, 1)
	thumbH = max(thumbH, 1)

	//nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
	thumbImg, err := d.rdpstate.ResizeCrop(
		uint16(bounds.Min.X), uint16(bounds.Min.Y),
		uint16(cropW), uint16(cropH),
		uint16(thumbW), uint16(thumbH),
	)
	if err != nil { //nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
		return nil, trace.Wrap(err, "resizing thumbnail crop")
	}

	d.buf.Reset()
	if err := d.pngEncoder.Encode(&d.buf, thumbImg); err != nil {
		return nil, trace.Wrap(err, "encoding thumbnail PNG")
	}

	return &pb.SessionRecordingThumbnail{
		CursorX:       int32(cursor.X),
		CursorY:       int32(cursor.Y),
		CursorVisible: cursor.Visible,
		ScreenWidth:   int32(screenW),
		ScreenHeight:  int32(screenH),
		Png:           bytes.Clone(d.buf.Bytes()),
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
