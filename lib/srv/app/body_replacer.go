/*
Copyright 2023 Gravitational, Inc.

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

package app

import (
	"bytes"
	"io"

	"github.com/gravitational/trace"
)

// bytesReplacingReader allows transparent replacement of a given token during read
// operation without requiring to read the entire content into memory.
type bytesReplacingReader struct {
	r          io.ReadCloser
	search     []byte
	searchLen  int
	replace    []byte
	replaceLen int
	delta      int // = replaceLen - searchLen. can be negative
	// err is the error that was encountered during the last Read operation.
	err        error
	buf        []byte
	buf0, buf1 int // buf[0:buf0]: bytes already processed; buf[buf0:buf1] bytes read in but not yet processed.
	max        int // because we need to replace 'search' with 'replace', this marks the max bytes we can read into buf
}

const defaultBufSize = int(4096)

// newBytesReplacingReader creates a new `*BytesReplacingReader`.
// `search` cannot be nil/empty. `replace` can.
func newBytesReplacingReader(r io.ReadCloser, search, replace []byte) (*bytesReplacingReader, error) {
	if r == nil {
		return nil, trace.BadParameter("reader cannot be nil")
	}
	if len(search) == 0 {
		return nil, trace.BadParameter("search cannot be empty")
	}
	bufLen := max(defaultBufSize, max(len(replace), len(search)))
	reader := &bytesReplacingReader{
		r:          r,
		search:     search,
		searchLen:  len(search),
		replace:    replace,
		replaceLen: len(replace),
		delta:      len(replace) - len(search),
		buf:        make([]byte, bufLen),
		max:        bufLen,
	}

	if reader.searchLen < reader.replaceLen {
		// If len(search) < len(replace), then we have to assume the worst case:
		// what's the max bound value such that if we have consecutive 'search' filling up
		// the buf up to buf[:max], and all of them are placed with 'replace', and the final
		// result won't end up exceed the len(buf)?
		reader.max = (len(reader.buf) / reader.replaceLen) * reader.searchLen
	}
	return reader, nil
}

// Read implements the `io.Reader` interface.
func (r *bytesReplacingReader) Read(p []byte) (int, error) {
	n := 0
	for {
		if r.buf0 > 0 {
			n = copy(p, r.buf[0:r.buf0])
			r.buf0 -= n
			r.buf1 -= n
			if r.buf1 == 0 && r.err != nil {
				return n, r.err
			}
			copy(r.buf, r.buf[n:r.buf1+n])
			return n, nil
		} else if r.err != nil {
			return 0, r.err
		}

		n, r.err = r.r.Read(r.buf[r.buf1:r.max])
		if n > 0 {
			r.buf1 += n
			for {
				index := bytes.Index(r.buf[r.buf0:r.buf1], r.search)
				if index < 0 {
					r.buf0 = max(r.buf0, r.buf1-r.searchLen+1)
					break
				}
				index += r.buf0
				copy(r.buf[index+r.replaceLen:r.buf1+r.delta], r.buf[index+r.searchLen:r.buf1])
				copy(r.buf[index:index+r.replaceLen], r.replace)
				r.buf0 = index + r.replaceLen
				r.buf1 += r.delta
			}
		}
		if r.err != nil {
			r.buf0 = r.buf1
		}
	}
}

func (r *bytesReplacingReader) Close() error {
	return r.r.Close()
}
