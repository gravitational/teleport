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

type contextKey string

const (
	// ModeratedSessionID is an optional parameter sent during SCP requests to specify which moderated session
	// to check for valid FileTransferRequests
	// used as a value in the file transfer context and env var for exec session
	ModeratedSessionID contextKey = "TELEPORT_MODERATED_SESSION_ID"
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

func (h *httpFS) Glob(ctx context.Context, _ string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return []string{h.fileName}, nil
}

func (h *httpFS) Stat(ctx context.Context, _ string) (fs.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return httpFileInfo{
		name: path.Base(h.fileName),
		size: h.fileSize,
	}, nil
}

func (h *httpFS) ReadDir(_ context.Context, _ string) ([]fs.FileInfo, error) {
	return nil, errDirsNotSupported
}

func (h *httpFS) Open(ctx context.Context, path string) (File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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

func (h *httpFS) Create(ctx context.Context, p string, size int64) (File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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

func (h *httpFS) Mkdir(_ context.Context, _ string) error {
	return errDirsNotSupported
}

func (h *httpFS) Chmod(_ context.Context, _ string, _ os.FileMode) error {
	return nil
}

func (h *httpFS) Chtimes(_ context.Context, _ string, _, _ time.Time) error {
	return nil
}

func (h *httpFS) Rename(_ context.Context, _, _ string) error {
	return nil
}

func (h *httpFS) Lstat(ctx context.Context, name string) (os.FileInfo, error) {
	return h.Stat(ctx, name)
}

func (h *httpFS) RemoveAll(_ context.Context, _ string) error {
	return errDirsNotSupported
}

func (h *httpFS) Link(_ context.Context, _, _ string) error {
	return nil
}

func (h *httpFS) Symlink(_ context.Context, _, _ string) error {
	return nil
}

func (h *httpFS) Remove(_ context.Context, _ string) error {
	return nil
}

func (h *httpFS) Chown(_ context.Context, _ string, _, _ int) error {
	return nil
}

func (h *httpFS) Truncate(_ context.Context, _ string, _ int64) error {
	return nil
}

func (h *httpFS) Readlink(_ context.Context, _ string) (string, error) {
	return "", nil
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
	return h.reader.Close()
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
