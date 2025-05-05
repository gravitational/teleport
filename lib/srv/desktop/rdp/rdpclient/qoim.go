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
	"encoding/binary"
	"io"
	"unsafe"
)

const opIndex = 0x40       // 01xxxxxx
const opDiff = 0x80        // 10xxxxxx
const opLuma = 0xc0        // 110xxxxx
const opRun = 0xc4         // 111xxxxx
const opExtendedRun = 0xfe // 11111110
const opRgb = 0xff         // 11111111

type pixel struct {
	value uint16
	r     uint16
	g     uint16
	b     uint16
}

func encode(data []byte, buf io.Writer) {
	l := len(data) / 2
	ptr := (*uint16)(unsafe.Pointer(&data[0]))
	pixels := unsafe.Slice(ptr, l)
	pxPrev := decode(0)
	hash := hashIndex(pxPrev)
	index := [64]uint16{}
	run := 0
	for _, p := range pixels {
		if p == pxPrev.value {
			run += 1
			continue
		}
		if run != 0 {
			pushRun(buf, hash, run)
			run = 0
		}
		px := decode(p)
		hash = hashIndex(px)
		if index[hash] == px.value {
			buf.Write([]byte{opIndex | hash})
		} else {
			index[hash] = px.value
			encodePixel(px, pxPrev, buf)
		}
		pxPrev = px
	}
	if run != 0 {
		pushRun(buf, hash, run)
	}
}

func encodePixel(px pixel, pxPrev pixel, buf io.Writer) {
	vg := px.g - pxPrev.g
	vg16 := vg + 16
	if vg16|31 == 31 {
		vr := px.r - pxPrev.r
		vb := px.b - pxPrev.b
		vgr := vr - vg
		vgb := vb - vg
		vr2, vg2, vb2 := vr+2, vg+2, vb+2
		if vr2|vg2|vb2|3 == 3 {
			buf.Write([]byte{byte(opDiff | (vr2 << 4) | (vg2 << 2) | vb2)})
			return
		}
		vgr8, vgb8 := vgr+8, vgb+8
		if px.value < 0x4000 && vgr8|vgb8|15 == 15 {
			buf.Write([]byte{byte(opLuma | vg16), byte((vgr8 << 4) | vgb8)})
			return
		}
	}
	if px.value < 0x4000 {
		buf.Write([]byte{byte(px.value >> 8), byte(px.value & 0xff)})
	} else {
		buf.Write([]byte{opRgb, byte(px.value >> 8), byte(px.value & 0xff)})
	}
}

func pushRun(buf io.Writer, hash byte, run int) {
	switch {
	case run == 1:
		buf.Write([]byte{opIndex | hash})
	case run > 30:
		tmp := make([]byte, binary.MaxVarintLen32+1)
		tmp[0] = opExtendedRun
		n := binary.PutUvarint(tmp[1:], uint64(run-31))
		buf.Write(tmp[:n+1])
	default:
		buf.Write([]byte{byte(opRun | (run - 1))})
	}
}

func hashIndex(px pixel) byte {
	return byte((px.r ^ px.g ^ px.b) % 64)
}

func decode(px uint16) pixel {
	return pixel{
		value: px,
		r:     px >> 11,
		g:     (px >> 5) & 0x3F,
		b:     px & 0x1F,
	}
}
