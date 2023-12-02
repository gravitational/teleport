/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"io"
)

// NewRepeatReader returns a repeat reader
func NewRepeatReader(repeat byte, count int) *RepeatReader {
	return &RepeatReader{
		repeat: repeat,
		count:  count,
	}
}

// RepeatReader repeats the same byte count times
// without allocating any data, the single instance
// of the repeat reader is not goroutine safe
type RepeatReader struct {
	repeat byte
	count  int
	read   int
}

// Read copies the same byte over and over to the data count times
func (r *RepeatReader) Read(data []byte) (int, error) {
	if r.read >= r.count {
		return 0, io.EOF
	}
	var copied int
	for i := 0; i < len(data); i++ {
		data[i] = r.repeat
		copied++
		r.read++
		if r.read >= r.count {
			break
		}
	}
	return copied, nil
}
