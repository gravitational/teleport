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
#cgo nocallback rdp_decoder_cursor_state
#cgo noescape rdp_decoder_cursor_state
#cgo nocallback rdp_decoder_cursor_bitmap
#cgo noescape rdp_decoder_cursor_bitmap
#cgo nocallback rdp_decoder_updated_regions_count
#cgo noescape rdp_decoder_updated_regions_count
#cgo nocallback rdp_decoder_updated_regions
#cgo noescape rdp_decoder_updated_regions
#cgo nocallback rdp_decoder_reset_updated_regions
#cgo noescape rdp_decoder_reset_updated_regions

#include <stdint.h>

typedef struct RdpDecoder RdpDecoder;

RdpDecoder* rdp_decoder_new(uint16_t width, uint16_t height, uint16_t io_channel_id, uint16_t user_channel_id);
void rdp_decoder_free(RdpDecoder* ptr);

void rdp_decoder_resize(RdpDecoder* ptr, uint16_t width, uint16_t height);
void rdp_decoder_process(RdpDecoder* ptr, const uint8_t* data, size_t len);
const uint8_t* rdp_decoder_image_data(RdpDecoder* ptr, uint16_t* width, uint16_t* height);
void rdp_decoder_cursor_state(RdpDecoder* ptr, uint8_t* out_visible, uint16_t* out_x, uint16_t* out_y);
const uint8_t* rdp_decoder_cursor_bitmap(RdpDecoder* ptr, uint16_t* out_width, uint16_t* out_height, uint16_t* out_hotspot_x, uint16_t* out_hotspot_y);
uint32_t rdp_decoder_updated_regions_count(RdpDecoder* ptr);
uint32_t rdp_decoder_updated_regions(RdpDecoder* ptr, uint16_t* out_buf, uint32_t max_count);
void rdp_decoder_reset_updated_regions(RdpDecoder* ptr);
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

func New(width, height uint16, opts ...Option) (*Decoder, error) {
	var config decoderConfig
	for _, opt := range opts {
		opt(&config)
	}

	ptr := C.rdp_decoder_new(
		C.uint16_t(width),
		C.uint16_t(height),
		C.uint16_t(config.ioChannelID),
		C.uint16_t(config.userChannelID),
	)
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

// CursorState returns the cursor position and visibility as tracked by the
// Rust decoder.
func (d *Decoder) CursorState() CursorState {
	if d == nil || d.ptr == nil {
		return CursorState{}
	}

	var outVisible C.uint8_t
	var outX, outY C.uint16_t

	C.rdp_decoder_cursor_state(d.ptr, &outVisible, &outX, &outY)

	return CursorState{
		Visible: outVisible != 0,
		X:       uint16(outX),
		Y:       uint16(outY),
	}
}

// CursorBitmap returns the current cursor bitmap, or nil if none is available.
func (d *Decoder) CursorBitmap() *CursorBitmapData {
	if d == nil || d.ptr == nil {
		return nil
	}

	var bmpW, bmpH, hotX, hotY C.uint16_t
	bmpData := C.rdp_decoder_cursor_bitmap(d.ptr, &bmpW, &bmpH, &hotX, &hotY)
	if bmpData == nil || bmpW == 0 || bmpH == 0 {
		return nil
	}

	w := int(bmpW)
	h := int(bmpH)

	cursorPix := make([]byte, w*h*4)
	copy(cursorPix, unsafe.Slice((*uint8)(bmpData), w*h*4))

	return &CursorBitmapData{
		Image: &image.RGBA{
			Pix:    cursorPix,
			Stride: w * 4,
			Rect:   image.Rect(0, 0, w, h),
		},
		HotspotX: int(hotX),
		HotspotY: int(hotY),
	}
}

// UpdatedRegions returns the individual screen regions updated since the last
// call to ResetUpdatedRegions. Each rectangle uses Go's exclusive Max convention
// (converted from the Rust decoder's inclusive coordinates).
func (d *Decoder) UpdatedRegions() []image.Rectangle {
	if d == nil || d.ptr == nil {
		return nil
	}

	count := int(C.rdp_decoder_updated_regions_count(d.ptr))
	if count == 0 {
		return nil
	}

	buf := make([]C.uint16_t, count*4)
	written := int(C.rdp_decoder_updated_regions(d.ptr, &buf[0], C.uint32_t(count)))

	regions := make([]image.Rectangle, written)
	for i := range written {
		base := i * 4
		// Rust uses inclusive right/bottom, Go uses exclusive — add 1.
		regions[i] = image.Rect(
			int(buf[base]),
			int(buf[base+1]),
			int(buf[base+2])+1,
			int(buf[base+3])+1,
		)
	}

	return regions
}

// ResetUpdatedRegions clears the accumulated update regions.
func (d *Decoder) ResetUpdatedRegions() {
	if d == nil || d.ptr == nil {
		return
	}
	C.rdp_decoder_reset_updated_regions(d.ptr)
}
