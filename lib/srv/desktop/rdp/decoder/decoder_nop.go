//go:build !desktop_access_rdp && !rust_rdp_decoder

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package decoder

import (
	"image"

	"github.com/gravitational/trace"
)

type Decoder struct{}

func New(width, height uint16, opts ...Option) (*Decoder, error) {
	return nil, trace.NotImplemented("the RDP decoder is not included in this build")
}

func (d *Decoder) Release()                          {}
func (d *Decoder) Resize(w, h uint16)                {}
func (d *Decoder) Process([]byte)                    {}
func (d *Decoder) Image() *image.RGBA                { return nil }
func (d *Decoder) CursorState() CursorState          { return CursorState{} }
func (d *Decoder) CursorBitmap() *CursorBitmapData   { return nil }
func (d *Decoder) UpdatedRegions() []image.Rectangle { return nil }
func (d *Decoder) ResetUpdatedRegions()              {}

func (d *Decoder) ResizeCrop(cropX, cropY, cropW, cropH, outW, outH uint16) *image.RGBA {
	return nil
}
func (d *Decoder) Dimensions() (uint16, uint16)        { return 0, 0 }
func (d *Decoder) SampleHash(sampleCount uint16) uint64 { return 0 }
