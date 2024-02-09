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

package sftp

import (
	"context"
	"io"
	"io/fs"
	"os"
)

// fileWrapper provides minimal required file interface for SFTP package
// including WriteTo() method required for concurrent data transfer.
type fileWrapper struct {
	file *os.File
}

func (wt *fileWrapper) Read(p []byte) (n int, err error) {
	return wt.file.Read(p)
}

func (wt *fileWrapper) Close() error {
	return wt.file.Close()
}

func (wt *fileWrapper) WriteTo(w io.Writer) (n int64, err error) {
	return io.Copy(w, wt.file)
}

func (wt *fileWrapper) Stat() (os.FileInfo, error) {
	return wt.file.Stat()
}

// fileStreamReader is a thin wrapper around fs.File with additional streams.
type fileStreamReader struct {
	ctx     context.Context
	streams []io.Reader
	file    fs.File
}

// Stat returns file stats.
func (r *fileStreamReader) Stat() (os.FileInfo, error) {
	return r.file.Stat()
}

// Read reads the data from a file and passes the read data to all readers.
// All errors from stream are returned except io.EOF.
func (r *fileStreamReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := r.file.Read(b)
	// Create a copy as not whole buffer can be filled.
	readBuff := b[:n]

	for _, stream := range r.streams {
		if _, innerError := stream.Read(readBuff); innerError != nil {
			// Ignore EOF
			if err != io.EOF {
				return 0, innerError
			}
		}
	}

	return n, err
}

// cancelWriter implements io.Writer interface with context cancellation.
type cancelWriter struct {
	ctx    context.Context
	stream io.Writer
}

func (c *cancelWriter) Write(b []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.stream.Write(b)
}
