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
	"slices"
	"testing"
)

type test struct {
	name   string
	input  []byte
	output []byte
}

func TestRun(t *testing.T) {
	for _, tt := range []test{
		{
			name:   "short run",
			input:  slices.Repeat([]byte{0, 0}, 4),
			output: []byte{0xe3},
		},
		{
			name:   "max short run",
			input:  slices.Repeat([]byte{0, 0}, 30),
			output: []byte{0xfd},
		},
		{
			name:   "extended byte 1",
			input:  slices.Repeat([]byte{0, 0}, 30+2),
			output: []byte{0xfe, 0x01},
		},
		{
			name:   "extended byte 2",
			input:  slices.Repeat([]byte{0, 0}, 30+128+2),
			output: []byte{0xfe, 0x81, 0x01},
		},
		{
			name:   "diff",
			input:  []byte{1, 0},
			output: []byte{0xab},
		},
		{
			name:   "negative diff",
			input:  []byte{255, 255},
			output: []byte{0x95},
		},
		{
			name:   "small red",
			input:  []byte{155, 0},
			output: []byte{0, 155},
		},
		{
			name:   "luma",
			input:  []byte{0x08, 0x41},
			output: []byte{0xd8, 0x88},
		},
		{
			name:   "negative luma",
			input:  []byte{0xbe, 0xf7},
			output: []byte{0xcd, 0x99},
		},
		{
			name:   "rgb",
			input:  []byte{0x18, 0xC3},
			output: []byte{0xff, 0xC3, 0x18},
		},
		{
			name:   "index",
			input:  []byte{1, 0, 0, 0},
			output: []byte{0xab, 0x40},
		},
		{
			name:   "color",
			input:  []byte{0x02, 0xae},
			output: []byte{0xff, 0xae, 0x02},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			encode(tt.input, &buf)
			assert.Equal(t, tt.output, buf.Bytes())
		})
	}
}
