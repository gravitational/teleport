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

package log

import (
	"io"
	"sync"
)

// SharedWriter is an [io.Writer] implementation that protects
// writes with a mutex. This allows a single [io.Writer] to be shared
// by both logrus and slog without their output clobbering each other.
type SharedWriter struct {
	mu sync.Mutex
	io.Writer
}

func (s *SharedWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Writer.Write(p)
}

// NewSharedWriter wraps the provided [io.Writer] in a writer that
// is thread safe.
func NewSharedWriter(w io.Writer) *SharedWriter {
	return &SharedWriter{Writer: w}
}
