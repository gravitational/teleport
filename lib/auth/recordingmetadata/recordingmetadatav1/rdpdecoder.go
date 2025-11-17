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

/*
#cgo linux,386 LDFLAGS: -L${SRCDIR}/../../../../target/i686-unknown-linux-gnu/release
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../../target/x86_64-unknown-linux-gnu/release
#cgo linux,arm LDFLAGS: -L${SRCDIR}/../../../../target/arm-unknown-linux-gnueabihf/release
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../../target/aarch64-unknown-linux-gnu/release
#cgo linux LDFLAGS: -l:librdp_client.a -lpthread -ldl -lm
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../../../target/x86_64-apple-darwin/release
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../../../target/aarch64-apple-darwin/release
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Security -lrdp_client -lpthread -ldl -lm
#cgo CFLAGS: -I${SRCDIR}/../../../srv/desktop/rdp/rdpclient

#include "librdpclient.h"
*/
import "C"
import (
	"image"
	"unsafe"

	"github.com/gravitational/trace"
)

type rdpDecoder struct {
	decoder *C.struct_RdpDecoder
	width   uint16
	height  uint16
}

func newRDPDecoder(width, height int) (*rdpDecoder, error) {
	ioChannelID := uint16(1003)
	userChannelID := uint16(1004)

	decoder := C.rdp_decoder_new(C.uint16_t(width), C.uint16_t(height), C.uint16_t(ioChannelID), C.uint16_t(userChannelID))
	if decoder == nil {
		return nil, trace.Errorf("failed to create RDP decoder")
	}

	return &rdpDecoder{
		decoder: decoder,
		width:   uint16(width),
		height:  uint16(height),
	}, nil
}

type decoderOutput struct {
	frameUpdate     *frameUpdate
	pointerUpdate   *pointerUpdate
	pointerPosition *pointerPosition
	pointerDefault  bool
	pointerHidden   bool
}

type frameUpdate struct {
	x, y uint16
	img  *image.RGBA
}

type pointerUpdate struct {
	width, height      uint16
	hotspotX, hotspotY uint16
	data               []byte
}

type pointerPosition struct {
	x, y uint16
}

func (r *rdpDecoder) decodeFastPathPDU(data []byte) (*decoderOutput, error) {
	if len(data) == 0 {
		return nil, nil
	}

	result := C.rdp_decoder_process(
		r.decoder,
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.uint32_t(len(data)),
	)
	defer C.rdp_decoder_free_result(result)

	if result.error_message != nil {
		errMsg := C.GoString(result.error_message)
		return nil, trace.Errorf("failed to process Fast Path PDU: %v", errMsg)
	}

	if result.outputs_len == 0 || result.outputs == nil {
		return nil, nil
	}

	outputs := (*[1 << 30]C.struct_CGOProcessorOutput)(unsafe.Pointer(result.outputs))[:result.outputs_len:result.outputs_len]

	do := &decoderOutput{}

	for i := 0; i < int(result.outputs_len); i++ {
		output := outputs[i]

		switch output.output_type {
		case C.GraphicsUpdate:
			update := output.frame_update

			if update.data == nil || update.data_len == 0 {
				continue
			}

			data := C.GoBytes(unsafe.Pointer(update.data), C.int(update.data_len))

			rect := image.Rect(
				int(update.x),
				int(update.y),
				int(update.x)+int(update.width),
				int(update.y)+int(update.height),
			)
			img := &image.RGBA{
				Pix:    data,
				Stride: int(update.width) * 4,
				Rect:   rect,
			}

			do.frameUpdate = &frameUpdate{
				img: img,
				x:   uint16(update.x),
				y:   uint16(update.y),
			}

		case C.PointerBitmap:
			update := output.pointer_update

			if update.data != nil && update.data_len > 0 {
				data := C.GoBytes(unsafe.Pointer(update.data), C.int(update.data_len))

				do.pointerUpdate = &pointerUpdate{
					width:    uint16(update.width),
					height:   uint16(update.height),
					hotspotX: uint16(update.hotspot_x),
					hotspotY: uint16(update.hotspot_y),
					data:     data,
				}
			}

		case C.PointerDefault:
			do.pointerDefault = true

		case C.PointerHidden:
			do.pointerHidden = true

		case C.PointerPosition:
			do.pointerPosition = &pointerPosition{
				x: uint16(output.pointer_x),
				y: uint16(output.pointer_y),
			}
		}
	}

	return do, nil
}

func (r *rdpDecoder) resize(width, height int) {
	r.width = uint16(width)
	r.height = uint16(height)

	C.rdp_decoder_resize(
		r.decoder,
		C.uint16_t(width),
		C.uint16_t(height),
	)
}
