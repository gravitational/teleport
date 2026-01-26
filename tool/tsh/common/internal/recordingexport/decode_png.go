/*
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

package recordingexport

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// PNGDecoder decodes screen fragments from legacy PNG desktop recordings.
type PNGDecoder struct {
	screen *image.NRGBA
}

func NewPNGDecoder(maxWidth, maxHeight int) *PNGDecoder {
	return &PNGDecoder{
		screen: image.NewNRGBA(image.Rectangle{
			Min: image.Pt(0, 0),
			Max: image.Pt(maxWidth, maxHeight),
		}),
	}
}

func (p *PNGDecoder) ClearScreen() {
	draw.Draw(p.screen, p.screen.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
}

func (p *PNGDecoder) UpdateScreen(msg tdp.Message) error {
	switch msg.(type) {
	case tdp.PNGFrame, tdp.PNG2Frame:
		fragment, err := imgFromPNGMessage(msg)
		if err != nil {
			return trace.Wrap(err)
		}
		draw.Draw(
			p.screen,
			rectFromPNGMessage(msg),
			fragment,
			fragment.Bounds().Min,
			draw.Src,
		)
	}
	return nil
}

func (p *PNGDecoder) Image() image.Image {
	return p.screen
}

func (p *PNGDecoder) Close() error { return nil }

func imgFromPNGMessage(msg tdp.Message) (image.Image, error) {
	switch msg := msg.(type) {
	case tdp.PNG2Frame:
		return png.Decode(bytes.NewReader(msg.Data()))
	case tdp.PNGFrame:
		return msg.Img, nil
	default:
		// this should never happen based on what we pass at the call site
		return nil, trace.BadParameter("unsupported TDP message %T", msg)
	}
}

func rectFromPNGMessage(msg tdp.Message) image.Rectangle {
	switch msg := msg.(type) {
	case tdp.PNG2Frame:
		return image.Rect(
			// add one to bottom and right dimension, as RDP bounds are inclusive
			int(msg.Left()), int(msg.Top()),
			int(msg.Right()+1), int(msg.Bottom()+1),
		)
	case tdp.PNGFrame:
		return msg.Img.Bounds()
	default:
		// this should never happen based on what we pass at the call site
		return image.Rectangle{}
	}
}
