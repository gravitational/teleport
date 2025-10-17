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

package integration

import (
	"bytes"
	"io"
)

// newSyncBuffer returns new in memory buffer
func newSyncBuffer() *syncBuffer {
	reader, writer := io.Pipe()
	buf := &bytes.Buffer{}
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		io.Copy(buf, reader)
	}()
	return &syncBuffer{
		reader:   reader,
		writer:   writer,
		buf:      buf,
		copyDone: copyDone,
	}
}

// syncBuffer is in memory bytes buffer that is
// safe for concurrent writes
type syncBuffer struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	buf    *bytes.Buffer
	// copyDone makes SyncBuffer.Close block until io.Copy goroutine finishes.
	// This prevents data races between io.Copy and SyncBuffer.String/Bytes.
	copyDone chan struct{}
}

func (b *syncBuffer) Write(data []byte) (n int, err error) {
	return b.writer.Write(data)
}

// String returns contents of the buffer
// after this call, all writes will fail
func (b *syncBuffer) String() string {
	b.Close()
	return b.buf.String()
}

// Bytes returns contents of the buffer
// after this call, all writes will fail
func (b *syncBuffer) Bytes() []byte {
	b.Close()
	return b.buf.Bytes()
}

// Close closes reads and writes on the buffer
func (b *syncBuffer) Close() error {
	err := b.reader.Close()
	err2 := b.writer.Close()

	// Explicitly wait for io.Copy goroutine to finish.
	<-b.copyDone

	if err != nil {
		return err
	}
	return err2
}
