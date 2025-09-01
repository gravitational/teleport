//go:build desktop_access_rdp

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

// RDPDecoder decodes RDP Fast Path PDUs into frame data
type RDPDecoder struct {
	decoder *C.struct_RdpDecoder
	width   uint16
	height  uint16
}

// NewRDPDecoder creates a new RDP decoder instance
func NewRDPDecoder(width, height uint16) (*RDPDecoder, error) {
	// For now, use dummy channel IDs since we're only decoding
	ioChannelID := uint16(1003)
	userChannelID := uint16(1004)
	
	decoder := C.rdp_decoder_new(C.uint16_t(width), C.uint16_t(height), C.uint16_t(ioChannelID), C.uint16_t(userChannelID))
	if decoder == nil {
		return nil, trace.Errorf("failed to create RDP decoder")
	}

	return &RDPDecoder{
		decoder: decoder,
		width:   width,
		height:  height,
	}, nil
}

// ProcessFastPathPDU processes an RDP Fast Path PDU and returns all decoder outputs
func (d *RDPDecoder) ProcessFastPathPDU(pduData []byte) (*DecoderOutput, error) {
	if len(pduData) == 0 {
		return nil, nil
	}

	result := C.rdp_decoder_process(
		d.decoder,
		(*C.uint8_t)(unsafe.Pointer(&pduData[0])),
		C.uint32_t(len(pduData)),
	)
	defer C.rdp_decoder_free_result(result)

	if result.error_message != nil {
		errMsg := C.GoString(result.error_message)
		return nil, trace.Errorf("failed to process Fast Path PDU: %s", errMsg)
	}

	if result.outputs_len == 0 || result.outputs == nil {
		// No updates in this PDU
		return nil, nil
	}

	// Process outputs - look for all types of updates
	outputs := (*[1 << 30]C.struct_CGOProcessorOutput)(unsafe.Pointer(result.outputs))[:result.outputs_len:result.outputs_len]
	
	decoderOutput := &DecoderOutput{}
	
	for i := 0; i < int(result.outputs_len); i++ {
		output := outputs[i]
		
		switch output.output_type {
		case C.GraphicsUpdate:
			frameUpdate := output.frame_update
			
			if frameUpdate.data == nil || frameUpdate.data_len == 0 {
				continue
			}
			
			// Copy the frame data to Go memory
			data := C.GoBytes(unsafe.Pointer(frameUpdate.data), C.int(frameUpdate.data_len))
			
			// Create RGBA image from the decoded data
			rect := image.Rect(
				int(frameUpdate.x), 
				int(frameUpdate.y), 
				int(frameUpdate.x)+int(frameUpdate.width), 
				int(frameUpdate.y)+int(frameUpdate.height),
			)
			img := &image.RGBA{
				Pix:    data,
				Stride: int(frameUpdate.width) * 4,
				Rect:   rect,
			}
			
			decoderOutput.FrameUpdate = &FrameUpdate{
				Image: img,
				X:     uint16(frameUpdate.x),
				Y:     uint16(frameUpdate.y),
			}
			
		case C.PointerBitmap:
			pointerUpdate := output.pointer_update
			
			if pointerUpdate.data != nil && pointerUpdate.data_len > 0 {
				// Copy the pointer bitmap data to Go memory
				data := C.GoBytes(unsafe.Pointer(pointerUpdate.data), C.int(pointerUpdate.data_len))
				
				decoderOutput.PointerUpdate = &PointerUpdate{
					Width:    uint16(pointerUpdate.width),
					Height:   uint16(pointerUpdate.height),
					HotspotX: uint16(pointerUpdate.hotspot_x),
					HotspotY: uint16(pointerUpdate.hotspot_y),
					Data:     data,
				}
			}
			
		case C.PointerDefault:
			decoderOutput.PointerDefault = true
			
		case C.PointerHidden:
			decoderOutput.PointerHidden = true
			
		case C.PointerPosition:
			decoderOutput.PointerPosition = &PointerPosition{
				X: uint16(output.pointer_x),
				Y: uint16(output.pointer_y),
			}
		}
	}
	
	// Return nil if no relevant updates
	if decoderOutput.FrameUpdate == nil && decoderOutput.PointerUpdate == nil && 
		decoderOutput.PointerPosition == nil && !decoderOutput.PointerDefault && !decoderOutput.PointerHidden {
		return nil, nil
	}
	
	return decoderOutput, nil
}

// Close destroys the decoder instance
func (d *RDPDecoder) Close() {
	if d.decoder != nil {
		C.rdp_decoder_free(d.decoder)
		d.decoder = nil
	}
}

// FrameUpdate represents a decoded frame update
type FrameUpdate struct {
	Image *image.RGBA
	X     uint16
	Y     uint16
}

// PointerUpdate represents a cursor/pointer update
type PointerUpdate struct {
	Width    uint16
	Height   uint16
	HotspotX uint16
	HotspotY uint16
	Data     []byte // RGBA bitmap data
}

// PointerPosition represents a cursor position update
type PointerPosition struct {
	X uint16
	Y uint16
}

// DecoderOutput represents any output from the decoder
type DecoderOutput struct {
	FrameUpdate     *FrameUpdate
	PointerUpdate   *PointerUpdate
	PointerPosition *PointerPosition
	PointerHidden   bool
	PointerDefault  bool
}
