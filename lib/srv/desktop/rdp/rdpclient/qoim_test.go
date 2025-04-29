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
	"github.com/stretchr/testify/assert"
	"image"
	"image/png"
	"os"
	"slices"
	"testing"
)

func TestTime(t *testing.T) {
	f, err := os.Open("/Users/hesperus/work/gg.png")
	assert.NoError(t, err)
	img, err := png.Decode(f)
	rgba := img.(*image.NRGBA)
	var buf bytes.Buffer
	buf.Grow(len(rgba.Pix) / 4)
	encode(rgba.Pix, &buf)
	assert.NoError(t, err)
}

func TestEncode(t *testing.T) {
	data := slices.Repeat([]byte{0, 0, 0, 0}, 62)
	var buf bytes.Buffer
	encode(data, &buf)
	assert.Equal(t, []byte{QoiOpRun | 61}, buf.Bytes())

	data = slices.Repeat([]byte{0, 0, 0, 0}, 127)
	buf.Reset()
	encode(data, &buf)
	assert.Equal(t, []byte{QoiOpExtendedRun, 127 - 63}, buf.Bytes())

	data = slices.Repeat([]byte{0, 0, 0, 0}, 128)
	buf.Reset()
	encode(data, &buf)
	assert.Equal(t, []byte{QoiOpExtendedRun, 65}, buf.Bytes())

	data = slices.Repeat([]byte{0, 0, 0, 0}, 63+128)
	buf.Reset()
	encode(data, &buf)
	assert.Equal(t, []byte{QoiOpExtendedRun, 128, 1}, buf.Bytes())
}
