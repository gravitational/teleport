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

package testercli

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/utils"
)

type recordType string

func (r recordType) color() int {
	switch r {
	case recordStdin:
		return 32 // green
	case recordStdout:
		return 34 // blue
	case recordStderr:
		return 33 // yellow
	default:
		return -1 // no color
	}
}

const (
	recordStdin  recordType = "stdin  (client->server)"
	recordStdout recordType = "stdout (server->client)"
	recordStderr recordType = "stderr   (command logs)"
)

type record struct {
	recordType recordType
	data       string
}

type recorder struct {
	mu          sync.Mutex
	records     []record
	enableColor bool
}

func (r *recorder) makeStdin(stdinPipe io.WriteCloser) io.WriteCloser {
	return writeCloserWrapper{
		Writer: utils.NewSyncWriter(io.MultiWriter(stdinPipe, r.makeRecorder(recordStdin))),
		Closer: stdinPipe,
	}
}

func (r *recorder) makeStdout(stdout io.ReadCloser) io.ReadCloser {
	return readCloserWrapper{
		Reader: io.TeeReader(stdout, r.makeRecorder(recordStdout)),
		Closer: stdout,
	}
}

func (r *recorder) makeStderr(stdout io.ReadCloser) io.ReadCloser {
	return readCloserWrapper{
		Reader: io.TeeReader(stdout, r.makeRecorder(recordStderr)),
		Closer: stdout,
	}
}

func (r *recorder) copyStderr(stderr io.Reader) {
	w := r.makeRecorder(recordStderr)
	go func() {
		io.Copy(w, stderr)
	}()
}

func (r *recorder) makeRecorder(recordType recordType) io.Writer {
	return writerFunc(func(p []byte) (int, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		record := record{
			recordType: recordType,
			data:       string(p),
		}
		r.records = append(r.records, record)
		return len(p), nil
	})
}

func (r *recorder) dump(w io.Writer) {
	for _, record := range r.records {
		r.dumpRecord(w, record)
	}
}

func (r *recorder) dumpRecord(w io.Writer, record record) {
	prefix := fmt.Sprintf("[%s]", record.recordType)
	if r.enableColor {
		prefix = fmt.Sprintf("\u001B[%dm%s\u001B[0m", record.recordType.color(), prefix)
	}
	fmt.Fprintln(w, prefix, strings.TrimSpace(record.data))
}

type writeCloserWrapper struct {
	io.Writer
	io.Closer
}

type readCloserWrapper struct {
	io.Reader
	io.Closer
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) {
	return w(p)
}
