/*
Copyright 2018 Gravitational, Inc.

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

package scp

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"

	"github.com/gravitational/trace"
)

const (
	// 644 means that files are readable and writeable by the owner of
	// the file and readable by users in the group owner of that file
	// and readable by everyone else.
	httpUploadFileMode = 0644
)

// HTTPTransferRequest describes HTTP file transfer request
type HTTPTransferRequest struct {
	// RemoteLocation is a destination location of the file
	RemoteLocation string
	// HTTPRequest is HTTP request
	HTTPRequest *http.Request
	// HTTPRequest is HTTP request
	HTTPResponse http.ResponseWriter
	// ProgressWriter is a writer for printing the progress
	Progress io.Writer
	// User is a user name
	User string
	// AuditLog is AuditLog log
	AuditLog events.IAuditLog
}

func (r *HTTPTransferRequest) parseRemoteLocation() (string, string, error) {
	dir, filename := filepath.Split(r.RemoteLocation)
	if filename == "" {
		return "", "", trace.BadParameter("failed to parse file remote location: %q", r.RemoteLocation)
	}

	return dir, filename, nil
}

// CreateHTTPUpload creates HTTP download command
func CreateHTTPUpload(req HTTPTransferRequest) (Command, error) {
	if req.HTTPRequest == nil {
		return nil, trace.BadParameter("missing parameter HTTPRequest")
	}

	dir, filename, err := req.parseRemoteLocation()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	contentLength := req.HTTPRequest.Header.Get("Content-Length")
	fileSize, err := strconv.ParseInt(contentLength, 10, 0)
	if err != nil {
		return nil, trace.BadParameter("failed to parse Content-Length header: %q", contentLength)
	}

	fs := &httpFileSystem{
		reader:   req.HTTPRequest.Body,
		fileName: filename,
		fileSize: fileSize,
	}

	flags := Flags{
		Target: []string{dir},
	}

	cfg := Config{
		User:           req.User,
		Flags:          flags,
		FileSystem:     fs,
		ProgressWriter: req.Progress,
		RemoteLocation: dir,
		AuditLog:       req.AuditLog,
	}

	cmd, err := CreateUploadCommand(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}

// CreateHTTPDownload creates HTTP upload command
func CreateHTTPDownload(req HTTPTransferRequest) (Command, error) {
	_, filename, err := req.parseRemoteLocation()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	flags := Flags{
		Target: []string{filename},
	}

	cfg := Config{
		Flags:          flags,
		User:           req.User,
		ProgressWriter: req.Progress,
		RemoteLocation: req.RemoteLocation,
		FileSystem: &httpFileSystem{
			writer: req.HTTPResponse,
		},
	}

	cmd, err := CreateDownloadCommand(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}

// httpFileSystem simulates file system calls while using HTTP response/request streams.
type httpFileSystem struct {
	writer   http.ResponseWriter
	reader   io.ReadCloser
	fileName string
	fileSize int64
}

// SetChmod sets file permissions. It does nothing as there are no permissions
// while processing HTTP downloads
func (l *httpFileSystem) SetChmod(path string, mode int) error {
	return nil
}

// MkDir creates a directory. This method is not implemented as creating directories
// is not supported during HTTP downloads.
func (l *httpFileSystem) MkDir(path string, mode int) error {
	return trace.BadParameter("directories are not supported in http file transfer")
}

// IsDir tells if this file is a directory. It always returns false as
// directories are not supported in HTTP file transfer
func (l *httpFileSystem) IsDir(path string) bool {
	return false
}

// OpenFile returns file reader
func (l *httpFileSystem) OpenFile(filePath string) (io.ReadCloser, error) {
	if l.reader == nil {
		return nil, trace.BadParameter("missing reader")
	}

	return l.reader, nil
}

// CreateFile sets proper HTTP headers and returns HTTP writer to stream incoming
// file content
func (l *httpFileSystem) CreateFile(filePath string, length uint64) (io.WriteCloser, error) {
	_, filename := filepath.Split(filePath)
	contentLength := strconv.FormatUint(length, 10)
	header := l.writer.Header()

	httplib.SetNoCacheHeaders(header)
	httplib.SetNoSniff(header)
	header.Set("Content-Length", contentLength)
	header.Set("Content-Type", "application/octet-stream")
	filename = url.QueryEscape(filename)
	header.Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%v"`, filename))

	return &nopWriteCloser{Writer: l.writer}, nil
}

// GetFileInfo returns file information
func (l *httpFileSystem) GetFileInfo(filePath string) (FileInfo, error) {
	return &httpFileInfo{
		name: l.fileName,
		path: l.fileName,
		size: l.fileSize,
	}, nil
}

// httpFileInfo is implementation of FileInfo interface used during HTTP
// file transfer
type httpFileInfo struct {
	path string
	name string
	size int64
}

// IsDir tells if this file in a directory
func (l *httpFileInfo) IsDir() bool {
	return false
}

// GetName returns file name
func (l *httpFileInfo) GetName() string {
	return l.name
}

// GetPath returns file path
func (l *httpFileInfo) GetPath() string {
	return l.path
}

// GetSize returns file size
func (l *httpFileInfo) GetSize() int64 {
	return l.size
}

// ReadDir returns an slice of files in the directory.
// This method is not supported in HTTP file transfer
func (l *httpFileInfo) ReadDir() ([]FileInfo, error) {
	return nil, trace.BadParameter("directories are not supported in http file transfer")
}

// GetModePerm returns file permissions that will be set on the
// file created on the remote host during HTTP upload.
func (l *httpFileInfo) GetModePerm() os.FileMode {
	return httpUploadFileMode
}

type nopWriteCloser struct {
	io.Writer
}

func (wr *nopWriteCloser) Close() error {
	return nil
}
