/*
 *
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
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
