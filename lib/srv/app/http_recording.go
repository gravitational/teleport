/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"io"
)

// chunkEmitter is called for each payload produced by a Read call.
// data is the raw bytes; index is the zero-based chunk sequence number;
// isLast is true for the final chunk.
type chunkEmitter func(data []byte, index int64, isLast bool)

// recordingBody wraps an io.ReadCloser and calls emit for every non-empty
// Read result. Chunk boundaries follow OS-level read sizes — no artificial
// splitting or buffering.
type recordingBody struct {
	inner   io.ReadCloser
	emit    chunkEmitter
	index   int64
	eofSeen bool
}

func newRecordingBody(inner io.ReadCloser, emit chunkEmitter) *recordingBody {
	return &recordingBody{inner: inner, emit: emit}
}

func (r *recordingBody) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		isLast := err == io.EOF
		if isLast {
			r.eofSeen = true
		}
		r.emit(p[:n], r.index, isLast)
		r.index++
	}
	return n, err
}

// Close closes the underlying body. If the body was abandoned before EOF,
// emits a final zero-byte chunk with isLast=true so consumers know the
// stream ended.
func (r *recordingBody) Close() error {
	if !r.eofSeen {
		r.emit(nil, r.index, true)
	}
	return r.inner.Close()
}
