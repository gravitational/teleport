/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package recordingmetadatav1

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
)

// memBuffer is an in-memory byte buffer that implements both io.Writer and
// io.WriterAt interfaces.
type memBuffer struct {
	buf manager.WriteAtBuffer
	// mu is a mutex to protect concurrent writes to the buffer. Even though the
	// underlying buffer is thread-safe, we need to prevent a race condition in
	// [memBuffer.Write] between checking the length of the buffer and writing to
	// it.
	mu sync.Mutex
}

func (b *memBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.WriteAt(p, int64(len(b.buf.Bytes())))
}

func (b *memBuffer) WriteAt(p []byte, pos int64) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.WriteAt(p, pos)
}

// Bytes return the underlying byte slice.
func (b *memBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}
