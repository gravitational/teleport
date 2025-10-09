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
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib"
)

const (
	// EnvModeratedSessionID is an optional env var parameter sent during SCP requests to specify which moderated session
	// to check for valid FileTransferRequests
	// used as a value in the file transfer context and env var for exec session
	EnvModeratedSessionID = "TELEPORT_MODERATED_SESSION_ID"
)

var errDirsNotSupported = trace.BadParameter("directories are not supported when transferring files over HTTP")

// httpFS provides API for accessing the a file over HTTP.
type httpFS struct {
	reader io.ReadCloser
	writer http.ResponseWriter

	fileName string
	fileSize int64
}

func (h *httpFS) Type() string {
	return "local"
}

func (h *httpFS) Glob(_ string) ([]string, error) {
	return []string{h.fileName}, nil
}

func (h *httpFS) Stat(_ string) (fs.FileInfo, error) {
	return httpFileInfo{
		name: path.Base(h.fileName),
		size: h.fileSize,
	}, nil
}

func (h *httpFS) ReadDir(_ string) ([]fs.FileInfo, error) {
	return nil, errDirsNotSupported
}

func (h *httpFS) Open(path string) (File, error) {
	if h.reader == nil {
		return nil, trace.BadParameter("missing reader")
	}

	return &httpFile{
		reader: h.reader,
		fileInfo: httpFileInfo{
			name: h.fileName,
			size: h.fileSize,
		},
	}, nil
}

func (h *httpFS) Create(p string, size int64) (File, error) {
	filename := path.Base(p)
	contentLength := strconv.FormatInt(size, 10)
	header := h.writer.Header()

	httplib.SetNoCacheHeaders(header)
	httplib.SetDefaultSecurityHeaders(header)
	header.Set("Content-Length", contentLength)
	header.Set("Content-Type", "application/octet-stream")
	filename = url.QueryEscape(filename)
	header.Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%s"`, filename))

	return &httpFile{
		writer: &nopWriteCloser{Writer: h.writer},
		fileInfo: httpFileInfo{
			name: filename,
			size: size,
		},
	}, nil
}

func (h *httpFS) OpenFile(p string, flags int) (File, error) {
	switch flags & 3 {
	case os.O_RDWR:
		return nil, trace.BadParameter("read-write files not supported for http")
	case os.O_RDONLY:
		return h.Open(p)
	case os.O_WRONLY:
		return h.Create(p, 0)
	default:
		return nil, trace.BadParameter("invalid flags")
	}
}

func (h *httpFS) Mkdir(_ string) error {
	return errDirsNotSupported
}

func (h *httpFS) Chmod(_ string, _ os.FileMode) error {
	return nil
}

func (h *httpFS) Chtimes(_ string, _, _ time.Time) error {
	return nil
}

func (h *httpFS) Rename(_, _ string) error {
	return nil
}

func (h *httpFS) Lstat(name string) (os.FileInfo, error) {
	return h.Stat(name)
}

func (h *httpFS) RemoveAll(_ string) error {
	return errDirsNotSupported
}

func (h *httpFS) Link(_, _ string) error {
	return nil
}

func (h *httpFS) Symlink(_, _ string) error {
	return nil
}

func (h *httpFS) Remove(_ string) error {
	return nil
}

func (h *httpFS) Chown(_ string, _, _ int) error {
	return nil
}

func (h *httpFS) Truncate(_ string, _ int64) error {
	return nil
}

func (h *httpFS) Readlink(_ string) (string, error) {
	return "", nil
}

func (h *httpFS) Getwd() (string, error) {
	return "", nil
}

func (h *httpFS) RealPath(path string) (string, error) {
	return path, nil
}

func (h *httpFS) Close() error {
	if h.reader != nil {
		return trace.Wrap(h.reader.Close())
	}
	return nil
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(w.Writer, r)
}

func (w *nopWriteCloser) Close() error {
	return nil
}

// httpFile implements [fs.File].
type httpFile struct {
	reader   io.ReadCloser
	writer   io.WriteCloser
	fileInfo httpFileInfo
}

func (h *httpFile) Read(p []byte) (int, error) {
	if h.reader == nil {
		return 0, trace.BadParameter("can't read from a file in write mode")
	}
	return h.reader.Read(p)
}

func (h *httpFile) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, trace.NotImplemented("can't seek in http files")
}

func (h *httpFile) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(h.writer, r)
}

func (h *httpFile) Write(p []byte) (int, error) {
	if h.writer == nil {
		return 0, trace.BadParameter("can't write to a file in read mode")
	}
	return h.writer.Write(p)
}

func (h *httpFile) WriteAt(_ []byte, _ int64) (int, error) {
	return 0, trace.NotImplemented("can't seek in http files")
}

func (h *httpFile) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, h)
}

func (h *httpFile) Stat() (fs.FileInfo, error) {
	return h.fileInfo, nil
}

func (h *httpFile) Name() string {
	return h.fileInfo.name
}

func (h *httpFile) Close() error {
	var errors []error
	if h.reader != nil {
		errors = append(errors, h.reader.Close())
	}
	if h.writer != nil {
		errors = append(errors, h.writer.Close())
	}
	return trace.NewAggregate(errors...)
}

// httpFileInfo is a simple implementation of [fs.FileMode] that only
// knows its file's name and size.
type httpFileInfo struct {
	name string
	size int64
}

func (h httpFileInfo) Name() string {
	return h.name
}

func (h httpFileInfo) Size() int64 {
	return h.size
}

func (h httpFileInfo) Mode() fs.FileMode {
	// return sensible default file permissions so when uploading files
	// the created destination file will have sensible permissions set
	return 0o644
}

func (h httpFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (h httpFileInfo) IsDir() bool {
	return false
}

func (h httpFileInfo) Sys() any {
	return nil
}
