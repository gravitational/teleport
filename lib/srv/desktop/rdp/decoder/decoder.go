//go:build desktop_access_rdp || rust_rdp_decoder

/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// Package decoder implements a RDP fast path decoder by calling into IronRDP via CGo.
//
// When used by Teleport (ie with the desktop_access_rdp build tag), this package links
// its symbols from librdp (the Rust library that Teleport already links).
//
// When used by other client tools (tsh), we link to a separate librdp_decoder library
// (using the rust_rdp_decoder build tag) to avoid linking in extra RDP code that we don't need.
//
// If neither build tag is specified, then a no-op implementation is used.
package decoder

/*
#cgo nocallback rdp_decoder_new
#cgo noescape rdp_decoder_new
#cgo nocallback rdp_decoder_free
#cgo noescape rdp_decoder_free
#cgo nocallback rdp_decoder_resize
#cgo noescape rdp_decoder_resize
#cgo nocallback rdp_decoder_process
#cgo noescape rdp_decoder_process
#cgo nocallback rdp_decoder_image_data
#cgo noescape rdp_decoder_image_data

#include <stdint.h>

typedef struct RdpDecoder RdpDecoder;

RdpDecoder* rdp_decoder_new(uint16_t width, uint16_t height);
void rdp_decoder_free(RdpDecoder* ptr);

void rdp_decoder_resize(RdpDecoder* ptr, uint16_t width, uint16_t height);
void rdp_decoder_process(RdpDecoder* ptr, const uint8_t* data, size_t len);
const uint8_t* rdp_decoder_image_data(RdpDecoder* ptr, uint16_t* width, uint16_t* height);
*/
import "C"

import (
	"errors"
	"image"
	"math"
	"unsafe"

	"golang.org/x/image/draw"
)

type Decoder struct {
	ptr *C.RdpDecoder
}

func New(width, height uint16) (*Decoder, error) {
	ptr := C.rdp_decoder_new(C.uint16_t(width), C.uint16_t(height))
	if ptr == nil {
		return nil, errors.New("failed to create decoder")
	}
	return &Decoder{ptr: ptr}, nil
}

func (d *Decoder) Release() {
	if d.ptr == nil {
		return
	}
	C.rdp_decoder_free(d.ptr)
	d.ptr = nil
}

func (d *Decoder) Resize(width, height uint16) {
	if d.ptr == nil {
		return
	}

	C.rdp_decoder_resize(d.ptr, C.uint16_t(width), C.uint16_t(height))
}

// Process processes an RDP fast path frame, updating the state of
// the decoder and its internal frame buffer.
func (d *Decoder) Process(frame []byte) {
	if d.ptr == nil {
		return
	}

	data := unsafe.SliceData(frame)
	C.rdp_decoder_process(d.ptr, (*C.uint8_t)(data), C.size_t(len(frame)))
}

// Image produces an RGBA image for the current state of the screen.
// Callers should check that the resulting image is not nil before
// attempting to operate on it.
func (d *Decoder) Image() *image.RGBA {
	if d == nil || d.ptr == nil {
		return nil
	}

	var outWidth, outHeight C.uint16_t
	data := C.rdp_decoder_image_data(d.ptr, &outWidth, &outHeight)
	if data == nil || outWidth == 0 || outHeight == 0 {
		return nil
	}

	rgba := image.NewRGBA(image.Rect(0, 0, int(outWidth), int(outHeight)))

	// Copy from the Rust-owned memory into Go memory.
	copy(rgba.Pix, unsafe.Slice((*uint8)(data), int(outWidth)*int(outHeight)*4))

	return rgba
}

// Thumbnail produces a scaled image of the current state of the screen.
// It uses a low-quality interpolator so it shouldn't be used for large
// size images.
func (d *Decoder) Thumbnail(width, height int) *image.RGBA {
	fullSize := d.Image()
	if fullSize == nil || width <= 0 || height <= 0 {
		return nil
	}

	srcBounds := fullSize.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW == 0 || srcH == 0 {
		return nil
	}

	// Compute scale to fit the source inside the requested thumbnail
	// while preserving aspect ratio.
	scaleW := float64(width) / float64(srcW)
	scaleH := float64(height) / float64(srcH)
	scale := math.Min(scaleW, scaleH)

	// Calculate destination size after scaling.
	dstW := int(math.Max(1, math.Floor(float64(srcW)*scale+0.5)))
	dstH := int(math.Max(1, math.Floor(float64(srcH)*scale+0.5)))

	thumbnail := image.NewRGBA(image.Rect(0, 0, width, height))

	// Center the scaled image within the thumbnail.
	offsetX := (width - dstW) / 2
	offsetY := (height - dstH) / 2
	dstRect := image.Rect(offsetX, offsetY, offsetX+dstW, offsetY+dstH)

	// Note: the nearest neighbor interpolator is fast, but produces the lowest quality
	// results. We're okay with this for thumbnails.
	draw.NearestNeighbor.Scale(thumbnail, dstRect, fullSize, srcBounds, draw.Over, nil)

	return thumbnail
}
