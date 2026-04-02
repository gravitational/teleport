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

package rdpstatetest

import (
	"encoding/binary"
	"slices"
)

const (
	// Bitmap Update Data (MS-RDPBCGR 2.2.9.1.1.3.1.2).
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/d681bb11-f3b5-4add-b092-19fe7075f9e3

	// BitmapUpdateType is the "update header" value for a bitmap update in a FastPath PDU.
	BitmapUpdateType = 0x0001

	// Client Fast-Path Input Event PDU actions (MS-RDPBCGR 2.2.8.1.2).
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/b8e7c588-51cb-455b-bb73-92d480903133

	// ActionFastPath is the "action" byte value for a FastPath PDU.
	ActionFastPath = 0x00

	// FastPath update codes and fragmentation (MS-RDPBCGR 2.2.9.1.2.1).
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/a1c4caa8-00ed-45bb-a06e-5177473766d3

	// SingleFragmentFlag is the bit flag to set in the update code byte of a FastPath PDU to indicate that the update is contained in a single fragment.
	SingleFragmentFlag = 0x00

	// UpdateCodeBitmap indicates a bitmap update fragment.
	UpdateCodeBitmap = 0x01
	// UpdateCodePointerHidden indicates a pointer hidden update fragment.
	UpdateCodePointerHidden = 0x05
	// UpdateCodePointerDefault indicates a pointer default update fragment.
	UpdateCodePointerDefault = 0x06
	// UpdateCodePointerPos indicates a pointer position update fragment.
	UpdateCodePointerPos = 0x08
	// UpdateCodeNewPointer indicates a new pointer update fragment.
	UpdateCodeNewPointer = 0x0b

	// RGB565 pixel values for test bitmaps.

	// RGB565White is a white pixel in RGB565 format.
	RGB565White = 0xFFFF
	// RGB565Red is a red pixel in RGB565 format.
	RGB565Red = 0xF800
	// RGB565Blue is a blue pixel in RGB565 format.
	RGB565Blue = 0x001F
)

// BGRARed is a BGRA pixel for a solid red color.
var BGRARed = [4]byte{0x00, 0x00, 0xFF, 0xFF}

// WrapFastPathUpdate wraps inner update data in a FastPath PDU envelope.
func WrapFastPathUpdate(updateCode byte, innerData []byte) []byte {
	// FastPathUpdatePdu
	fpUpdate := make([]byte, 3)
	fpUpdate[0] = updateCode | SingleFragmentFlag
	binary.LittleEndian.PutUint16(fpUpdate[1:], uint16(len(innerData)))

	// FastPathHeader
	body := slices.Concat(fpUpdate, innerData)
	totalLen := 2 + len(body)
	header := []byte{ActionFastPath, byte(totalLen)}

	return slices.Concat(header, body)
}

// BuildBitmapPDU constructs a minimal valid RDP fast path PDU containing an uncompressed 16bpp bitmap update.
// The bitmap fills a rectangle at (left, top) with dimensions w x h using the given RGB565 pixel value.
func BuildBitmapPDU(left, top, w, h int, rgb565 uint16) []byte {
	bitmapData := make([]byte, 18)
	binary.LittleEndian.PutUint16(bitmapData[0:], uint16(left))
	binary.LittleEndian.PutUint16(bitmapData[2:], uint16(top))
	binary.LittleEndian.PutUint16(bitmapData[4:], uint16(left+w-1)) // right (inclusive)
	binary.LittleEndian.PutUint16(bitmapData[6:], uint16(top+h-1))  // bottom (inclusive)
	binary.LittleEndian.PutUint16(bitmapData[8:], uint16(w))
	binary.LittleEndian.PutUint16(bitmapData[10:], uint16(h))
	binary.LittleEndian.PutUint16(bitmapData[12:], 16) // bitsPerPixel
	binary.LittleEndian.PutUint16(bitmapData[14:], 0)  // flags (uncompressed)

	rowBytes := w * 2 // 2 bytes per pixel at 16bpp
	if rowBytes%4 != 0 {
		// rows must be 4-byte aligned
		rowBytes += 4 - (rowBytes % 4)
	}

	pixelDataLen := rowBytes * h
	binary.LittleEndian.PutUint16(bitmapData[16:], uint16(pixelDataLen))

	pixelData := make([]byte, pixelDataLen)
	for row := range h {
		for col := range w {
			binary.LittleEndian.PutUint16(pixelData[row*rowBytes+col*2:], rgb565)
		}
	}

	// BitmapUpdateData header
	bitmapUpdate := make([]byte, 4)
	binary.LittleEndian.PutUint16(bitmapUpdate[0:], BitmapUpdateType)
	binary.LittleEndian.PutUint16(bitmapUpdate[2:], 1) // number of rectangles

	innerData := slices.Concat(bitmapUpdate, bitmapData, pixelData)

	return WrapFastPathUpdate(UpdateCodeBitmap, innerData)
}

// BuildNewPointerPDU constructs a FastPath "New Pointer" update with a solid-color 32bpp cursor of the given dimensions
// and BGRA pixel value.
// The resulting DecodedPointer bitmap will be in RGBA format after IronRDP decoding.
func BuildNewPointerPDU(w, h, hotspotX, hotspotY int, bgra [4]byte) []byte {
	// XOR mask: 32bpp BGRA pixels, bottom-up scanlines. For 32bpp the stride is always w*4 (always 16-bit aligned).
	xorStride := w * 4
	xorMask := make([]byte, xorStride*h)
	for row := range h {
		for col := range w {
			off := row*xorStride + col*4
			xorMask[off] = bgra[0]
			xorMask[off+1] = bgra[1]
			xorMask[off+2] = bgra[2]
			xorMask[off+3] = bgra[3]
		}
	}

	// TS_POINTERATTRIBUTE (New Pointer): xor_bpp + ColorPointerAttribute
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/584e4438-574c-45f4-947f-b0edcd9ae32c
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[0:], 32)               // xor_bpp
	binary.LittleEndian.PutUint16(data[2:], 0)                // cache_index
	binary.LittleEndian.PutUint16(data[4:], uint16(hotspotX)) // hot_spot.x
	binary.LittleEndian.PutUint16(data[6:], uint16(hotspotY)) // hot_spot.y
	binary.LittleEndian.PutUint16(data[8:], uint16(w))        // width
	binary.LittleEndian.PutUint16(data[10:], uint16(h))       // height
	binary.LittleEndian.PutUint16(data[12:], 0)               // and_mask_len (empty → default opaque)
	binary.LittleEndian.PutUint16(data[14:], uint16(len(xorMask)))

	return WrapFastPathUpdate(UpdateCodeNewPointer, slices.Concat(data, xorMask))
}

// BuildPointerPositionPDU constructs a FastPath "Pointer Position" update.
func BuildPointerPositionPDU(x, y int) []byte {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint16(data[0:], uint16(x))
	binary.LittleEndian.PutUint16(data[2:], uint16(y))

	return WrapFastPathUpdate(UpdateCodePointerPos, data)
}

// BuildPointerHiddenPDU constructs a FastPath "Pointer Hidden" update.
func BuildPointerHiddenPDU() []byte {
	return WrapFastPathUpdate(UpdateCodePointerHidden, nil)
}

// BuildPointerDefaultPDU constructs a FastPath "Pointer Default" update.
func BuildPointerDefaultPDU() []byte {
	return WrapFastPathUpdate(UpdateCodePointerDefault, nil)
}
