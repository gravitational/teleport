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
	"io/fs"
	"os"
	"sync"

	"github.com/gravitational/trace"
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

// FileSharedWriter is similar to SharedWriter except that it requires a os.File instead of a io.Writer.
// This is to allow the File reopen required by logrotate and similar tools.
// SharedWriter must be used for log destinations that don't have the reopen requirement, like stdout and stderr.
// This is thread safe.
type FileSharedWriter struct {
	*os.File
	fileFlag int
	fileMode fs.FileMode
	mu       sync.Mutex
}

func (s *FileSharedWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.File.Write(p)
}

// Reopen closes the file and opens it again using APPEND mode.
func (s *FileSharedWriter) Reopen() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Close(); err != nil {
		return trace.Wrap(err)
	}

	s.File, err = os.OpenFile(s.Name(), s.fileFlag, s.fileMode)
	return trace.Wrap(err)
}

// NewFileSharedWriter wraps the provided [os.File] in a writer that is thread safe.
func NewFileSharedWriter(f *os.File, flag int, mode fs.FileMode) *FileSharedWriter {
	return &FileSharedWriter{File: f, fileFlag: flag, fileMode: mode}
}
