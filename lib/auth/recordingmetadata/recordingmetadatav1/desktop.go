/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

	"github.com/gravitational/trace"
	"golang.org/x/image/draw"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

const (
	thumbnailWidth  = 1000
	thumbnailHeight = 600
)

// desktopThumbnailGenerator generates thumbnails for a desktop session.
type desktopThumbnailGenerator struct {
	decoder *rdpDecoder

	currentFrame   *image.RGBA
	contentBounds  image.Rectangle
	thumbnailIndex int

	cursorX      uint16
	cursorY      uint16
	cursorHidden bool
}

func newDesktopThumbnailGenerator() (*desktopThumbnailGenerator, error) {
	decoder, err := newRDPDecoder(0, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &desktopThumbnailGenerator{decoder: decoder}, nil
}

func (d *desktopThumbnailGenerator) HandleEvent(evt apievents.AuditEvent) error {
	switch evt := evt.(type) {
	case *apievents.DesktopRecording:
		msg, err := tdp.Decode(evt.Message)
		if err != nil {
			return trace.Wrap(err, "decoding TDP message")
		}

		switch m := msg.(type) {
		case tdp.RDPFastPathPDU:
			return d.handleFastPathPDU(m)
		}
	default:
		return trace.BadParameter("unsupported event type: %T", evt)
	}
	return nil
}

func (d *desktopThumbnailGenerator) Resize(width, height int) {
	d.decoder.resize(width, height)
}

func (d *desktopThumbnailGenerator) ProduceThumbnail() *pb.SessionRecordingThumbnail {
	if d.currentFrame == nil || d.contentBounds.Empty() {
		return &pb.SessionRecordingThumbnail{}
	}

	srcHeight := d.contentBounds.Dy()
	scale := float64(thumbnailHeight) / float64(srcHeight)

	scaledWidth := int(float64(d.contentBounds.Dx()) * scale)

	thumbImg := image.NewRGBA(image.Rect(0, 0, thumbnailWidth, thumbnailHeight))

	if scaledWidth <= thumbnailWidth {
		xOffset := (thumbnailWidth - scaledWidth) / 2
		dstRect := image.Rect(xOffset, 0, xOffset+scaledWidth, thumbnailHeight)
		draw.CatmullRom.Scale(thumbImg, dstRect, d.currentFrame, d.contentBounds, draw.Over, nil)
	} else {
		tempImg := image.NewRGBA(image.Rect(0, 0, scaledWidth, thumbnailHeight))
		draw.CatmullRom.Scale(tempImg, tempImg.Bounds(), d.currentFrame, d.contentBounds, draw.Over, nil)

		srcRect := image.Rect(0, 0, thumbnailWidth, thumbnailHeight)
		draw.Draw(thumbImg, thumbImg.Bounds(), tempImg, srcRect.Min, draw.Src)
	}

	d.thumbnailIndex++

	var buf bytes.Buffer
	if err := png.Encode(&buf, thumbImg); err != nil {
		return &pb.SessionRecordingThumbnail{}
	}

	// Save thumbnail to file for debugging
	//filename := fmt.Sprintf("thumbnail_%d.png", d.thumbnailIndex)
	//if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
	//	return &pb.SessionRecordingThumbnail{}
	//}

	return &pb.SessionRecordingThumbnail{
		Cols:          int32(thumbnailWidth),
		Rows:          int32(thumbnailHeight),
		CursorX:       int32(d.cursorX),
		CursorY:       int32(d.cursorY),
		CursorVisible: !d.cursorHidden,
		Png:           buf.Bytes(),
	}
}

func (d *desktopThumbnailGenerator) handleFastPathPDU(evt tdp.RDPFastPathPDU) error {
	output, err := d.decoder.decodeFastPathPDU(evt)
	if err != nil {
		return trace.Wrap(err, "decoding RDP Fast-Path PDU")
	}

	if output == nil {
		return nil
	}

	if output.frameUpdate != nil {
		d.applyFrameUpdate(output.frameUpdate)
	}

	if output.pointerUpdate != nil {
		d.cursorHidden = false
	}

	if output.pointerPosition != nil {
		d.cursorX = output.pointerPosition.x
		d.cursorY = output.pointerPosition.y
	}

	if output.pointerHidden {
		d.cursorHidden = true
	}

	if output.pointerDefault {
		d.cursorHidden = false
	}

	return nil
}

func (d *desktopThumbnailGenerator) applyFrameUpdate(update *frameUpdate) {
	if update == nil || update.img == nil {
		return
	}

	bounds := update.img.Bounds()

	if d.currentFrame == nil {
		d.currentFrame = image.NewRGBA(image.Rect(0, 0, bounds.Max.X, bounds.Max.Y))
		d.contentBounds = bounds
	} else if bounds.Max.X > d.currentFrame.Bounds().Max.X || bounds.Max.Y > d.currentFrame.Bounds().Max.Y {
		newWidth := max(d.currentFrame.Bounds().Max.X, bounds.Max.X)
		newHeight := max(d.currentFrame.Bounds().Max.Y, bounds.Max.Y)

		newFrame := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		draw.Draw(newFrame, d.currentFrame.Bounds(), d.currentFrame, image.Point{}, draw.Src)
		d.currentFrame = newFrame
	}

	draw.Draw(d.currentFrame, bounds, update.img, bounds.Min, draw.Src)

	if d.contentBounds.Empty() {
		d.contentBounds = bounds
	} else {
		d.contentBounds = d.contentBounds.Union(bounds)
	}
}
