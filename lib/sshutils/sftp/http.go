/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	// FileTransferRequestID is an optional parameter id of an file transfer request that has gone through
	// an approval process during a moderated session to allow a file transfer scp command to be executed
	// used as a value in the file transfer context and env var for exec session
	FileTransferRequestID contextKey = "FILE_TRANSFER_REQUEST_ID"

	// ModeratedSessionID is an optional parameter sent during SCP requests to specify which moderated session
	// to check for valid FileTransferRequests
	// used as a value in the file transfer context and env var for exec session
	ModeratedSessionID contextKey = "MODERATED_SESSION_ID"
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

func (h *httpFS) Open(ctx context.Context, path string) (fs.File, error) {
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

func (h *httpFS) Create(ctx context.Context, p string, size int64) (io.WriteCloser, error) {
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

	return &nopWriteCloser{
		Writer: h.writer,
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
	fileInfo httpFileInfo
}

func (h *httpFile) Read(p []byte) (int, error) {
	return h.reader.Read(p)
}

func (h *httpFile) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, h)
}

func (h *httpFile) Stat() (fs.FileInfo, error) {
	return h.fileInfo, nil
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
