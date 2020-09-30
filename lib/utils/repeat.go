/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
