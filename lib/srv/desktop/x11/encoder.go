package x11

import "unsafe"

const (
	qoiIndex byte = 0b00_000000
	qoiDiff  byte = 0b01_000000
	qoiLuma  byte = 0b10_000000
	qoiRun   byte = 0b11_000000
	qoiRgb   byte = 0b1111_1110
)

const qoiMagic = "qoif"

func hash(r, g, b byte) byte {
	return (r*3 + g*5 + b*7 + 0xF5 /*byte(255*11)*/) % 64
}

type pixel [4]byte

func encodeQOI(data []byte) []byte {
	var index [64]pixel
	pxPrev := pixel{0, 0, 0, 255}
	run := 0
	out := 0

	p := &data[0]

	for i := 0; i < len(data); i += 4 {
		px := pixel{data[0], data[1], data[2], 0xFF}
		if px == pxPrev {
			run++
			lastPixel := i == len(data)-4
			if run == 62 || lastPixel {
				data[out] = qoiRun | byte(run-1)
				out++
				run = 0
			}
			continue
		}
		if run > 0 {
			data[out] = qoiRun | byte(run-1)
			out++
			run = 0
		}
		pos := hash(px[0], px[1], px[2])
		if index[pos] == px {
			data[out] = qoiIndex | pos
			out++
		} else {
			index[pos] = px

			vr := int8(int(px[0]) - int(pxPrev[0]))
			vg := int8(int(px[1]) - int(pxPrev[1]))
			vb := int8(int(px[2]) - int(pxPrev[2]))

			vgr := vr - vg
			vgb := vb - vg

			if vr > -3 && vr < 2 && vg > -3 && vg < 2 && vb > -3 && vb < 2 {
				data[out] = qoiDiff | byte((vr+2)<<4|(vg+2)<<2|(vb+2))
				out++
			} else if vgr > -9 && vgr < 8 && vg > -33 && vg < 32 && vgb > -9 && vgb < 8 {
				data[out] = qoiLuma | byte(vg+32)
				data[out+1] = byte((vgr+8)<<4) | byte(vgb+8)
				out += 2
			} else {
				data[out] = qoiRgb
				copy(data[out+1:], px[:3])
				out += 4
			}
		}

		pxPrev = px
	}
	return data[0:out]
}
