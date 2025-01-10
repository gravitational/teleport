/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package log

import (
	"io"
	"runtime"
)

// WriterFinalizer is a wrapper for the [io.WriteCloser] to automate resource cleanup.
type WriterFinalizer[T io.WriteCloser] struct {
	writer T
}

// NewWriterFinalizer wraps the provided writer [io.WriteCloser] to trigger Close function
// after writer is unassigned from any variable.
func NewWriterFinalizer[T io.WriteCloser](writer T) *WriterFinalizer[T] {
	wr := &WriterFinalizer[T]{
		writer: writer,
	}
	runtime.SetFinalizer(wr, (*WriterFinalizer[T]).Close)
	return wr
}

// Write writes len(b) bytes from b to the writer.
func (w *WriterFinalizer[T]) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

// Close wraps closing function of internal writer.
func (w *WriterFinalizer[T]) Close() error {
	return w.writer.Close()
}
