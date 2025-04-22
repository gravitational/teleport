/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package rdpclient

import (
	"bytes"
	"encoding/binary"
	"slices"
)

const QoiOpIndex = 0x00       // 00xxxxxx
const QoiOpDiff = 0x40        // 01xxxxxx
const QoiOpLuma = 0x80        // 10xxxxxx
const QoiOpRun = 0xc0         // 11xxxxxx
const QoiOpRgb = 0xfe         // 11111110
const QoiOpExtendedRun = 0xff // 11111111

func encode(data []byte, buf *bytes.Buffer) {
	pxPrev := []byte{0, 0, 0, 0}
	hashPrev := hashIndex(pxPrev)
	index := [64][]byte{}
	indexAllowed := false
	run := 0
	for px := range slices.Chunk(data, 4) {
		if slices.Equal(px, pxPrev) {
			run += 1
		} else {
			if run != 0 {
				pushRun(buf, hashPrev, indexAllowed, run)
				run = 0
			}
			indexAllowed = true
			hashPrev = hashIndex(px)
			indexPx := index[hashPrev]
			if slices.Equal(indexPx, px[:3]) {
				buf.WriteByte(QoiOpIndex | hashPrev)
			} else {
				index[hashPrev] = px[:3]
				encodePixel(px, pxPrev, buf)
			}
			pxPrev = px
		}
	}
	if run != 0 {
		pushRun(buf, hashPrev, indexAllowed, run)
	}
}

func encodePixel(data []byte, pxPrev []byte, buf *bytes.Buffer) {
	vg := data[1] - pxPrev[1]
	vg32 := vg + 32
	if vg32|63 == 63 {
		vr := data[2] - pxPrev[2]
		vb := data[0] - pxPrev[0]
		vgr := vr - vg
		vgb := vb - vg
		vr2, vg2, vb2 := vr+2, vg+2, vb+2
		if vr2|vg2|vb2|3 == 3 {
			buf.WriteByte(QoiOpDiff | (vr2 << 4) | (vg2 << 2) | vb2)
		} else {
			vgr8, vgb8 := vgr+8, vgb+8
			if vgr8|vgb8|15 == 15 {
				buf.WriteByte(QoiOpLuma | vg32)
				buf.WriteByte((vgr8 << 4) | vgb8)
			} else {
				buf.WriteByte(QoiOpRgb)
				buf.WriteByte(data[2])
				buf.WriteByte(data[1])
				buf.WriteByte(data[0])
			}
		}
	} else {
		buf.WriteByte(QoiOpRgb)
		buf.WriteByte(data[2])
		buf.WriteByte(data[1])
		buf.WriteByte(data[0])
	}
}

func pushRun(buf *bytes.Buffer, hashPrev byte, indexAllowed bool, run int) {
	if run == 1 && indexAllowed {
		buf.WriteByte(QoiOpIndex | hashPrev)
	} else if run > 62 {
		buf.WriteByte(QoiOpExtendedRun)
		buf.Write(binary.AppendUvarint(nil, uint64(run-63)))
	} else {
		buf.WriteByte(byte(QoiOpRun | (run - 1)))
	}
}

func hashIndex(data []byte) byte {
	return (data[0] ^ data[1] ^ data[2]) % 64
}
