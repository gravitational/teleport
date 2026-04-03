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

// Package sftp handles file transfers client-side via SFTP.
package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path" // SFTP requires UNIX-style path separators
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils/scp"
)

// FileTransferRequest holds the settings for an SFTP file transfer.
type FileTransferRequest struct {
	// Sources is the source paths, local or remtoe.
	Sources Sources
	// Destination is the destination path, local or remote.
	Destination Target
	// DialHost opens an SSH client to the given address.
	DialHost func(ctx context.Context, login, addr string) (*ssh.Client, error)
	// Recursive indicates recursive file transfer.
	Recursive bool
	// PreserveAttrs preserves access and modification times
	// from the original file.
	PreserveAttrs bool
	// ProgressWriter is used to write the progress output.
	ProgressWriter io.Writer
	// ProgressStream is a callback to return a read/writer for printing the progress
	// (used only on the client)
	ProgressStream func(fileInfo os.FileInfo) io.ReadWriter
	// Log optionally specifies the logger
	Log *slog.Logger
	// HTTPReader is a reader for downloads from an HTTP server.
	HTTPReader io.ReadCloser
	// HTTPWriter is a writer for uploads to an HTTP server.
	HTTPWriter http.ResponseWriter
	// Size is the size of the file for HTTP transfers.
	Size int64
	// ModeratedSessionID is the optional ID of a moderated session.
	ModeratedSessionID string

	srcFS FileSystem
	dstFS FileSystem
}

func (req *FileTransferRequest) checkAndSetDefaults() error {
	if len(req.Sources.Paths) == 0 {
		return trace.BadParameter("missing sources")
	}
	if req.Sources.Addr != nil && req.Sources.Login == "" {
		return trace.BadParameter("missing login for source host")
	}
	if req.Destination.Addr != nil && req.Destination.Login == "" {
		return trace.BadParameter("missing login for destination host")
	}
	if (req.Sources.Addr != nil || req.Destination.Addr != nil) && req.DialHost == nil {
		return trace.BadParameter("request has a remote target but DialHost is not set")
	}

	if req.Log == nil {
		req.Log = slog.Default()
	}
	req.Log = req.Log.With(
		teleport.ComponentKey, "SFTP",
		"src_paths", req.Sources.Paths,
		"dst_path", req.Destination,
		"recursive", req.Recursive,
		"preserve_attrs", req.PreserveAttrs,
	)
	if req.ProgressWriter != nil && req.ProgressStream == nil {
		req.ProgressStream = func(fileInfo os.FileInfo) io.ReadWriter {
			return newProgressBar(fileInfo.Size(), fileInfo.Name(), req.ProgressWriter)
		}
	}
	return nil
}

// HTTPTransferRequest describes file transfer request over HTTP.
type HTTPTransferRequest struct {
	// Src is the source file name
	Src Target
	// Dst is the destination file name
	Dst Target
	// HTTPRequest is where the source file will be read from for
	// file upload transfers
	HTTPRequest *http.Request
	// HTTPResponse is where the destination file will be written to for
	// file download transfers
	HTTPResponse http.ResponseWriter
	// DialHost opens an SSH client to the given address.
	DialHost func(ctx context.Context, login, addr string) (*ssh.Client, error)
	// ModeratedSessionID is the optional ID of a moderated session.
	ModeratedSessionID string
}

// CreateHTTPUploadRequest returns a FileTransferRequest ready to upload a file from
// a HTTP request over SFTP.
func CreateHTTPUploadRequest(req HTTPTransferRequest) (*FileTransferRequest, error) {
	if err := req.checkDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.HTTPRequest == nil {
		return nil, trace.BadParameter("HTTP request is empty")
	}

	contentLength := req.HTTPRequest.Header.Get("Content-Length")
	fileSize, err := strconv.ParseInt(contentLength, 10, 0)
	if err != nil {
		return nil, trace.Errorf("failed to parse Content-Length header: %w", err)
	}
	return &FileTransferRequest{
		Sources: Sources{
			Login: req.Src.Login,
			Addr:  req.Src.Addr,
			Paths: []string{req.Src.Path},
		},
		Destination:        req.Dst,
		DialHost:           req.DialHost,
		HTTPReader:         req.HTTPRequest.Body,
		Size:               fileSize,
		ModeratedSessionID: req.ModeratedSessionID,
	}, nil
}

// CreateHTTPDownloadRequest returns a FileTransferRequest ready to download a file
// from over SFTP and write it to a HTTP response.
func CreateHTTPDownloadRequest(req HTTPTransferRequest) (*FileTransferRequest, error) {
	if err := req.checkDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.HTTPResponse == nil {
		return nil, trace.BadParameter("HTTP response is empty")
	}

	return &FileTransferRequest{
		Sources: Sources{
			Login: req.Src.Login,
			Addr:  req.Src.Addr,
			Paths: []string{req.Src.Path},
		},
		Destination:        req.Dst,
		DialHost:           req.DialHost,
		HTTPWriter:         req.HTTPResponse,
		ModeratedSessionID: req.ModeratedSessionID,
	}, nil
}

func (h HTTPTransferRequest) checkDefaults() error {
	if h.Src.Path == "" {
		return trace.BadParameter("source path is empty")
	}
	if h.Dst.Path == "" {
		return trace.BadParameter("destination path is empty")
	}
	return nil
}

// TransferFiles transfers files from the configured source paths to the
// configured destination path over SFTP or HTTP depending on the Config.
func TransferFiles(ctx context.Context, req *FileTransferRequest) error {
	if err := req.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	// Set up file systems.
	switch {
	case req.srcFS != nil:
	case req.HTTPReader != nil:
		if len(req.Sources.Paths) > 1 {
			return trace.BadParameter("only one source allowed for http filesystems")
		}
		req.srcFS = &httpFS{
			reader:   req.HTTPReader,
			fileName: req.Sources.Paths[0],
			fileSize: req.Size,
		}
	case req.Sources.Addr != nil:
		sshClient, err := req.DialHost(ctx, req.Sources.Login, req.Sources.Addr.String())
		if err != nil {
			return trace.Wrap(err)
		}
		req.srcFS, err = OpenRemoteFilesystem(ctx, sshClient, req.ModeratedSessionID)
		if err != nil {
			return trace.Wrap(err)
		}
		for i, srcPath := range req.Sources.Paths {
			expandedPath, err := ExpandHomeDir(srcPath)
			if err != nil {
				return trace.Wrap(err)
			}
			req.Sources.Paths[i] = expandedPath
		}
	default:
		req.srcFS = localFS{}
	}
	defer req.srcFS.Close()

	switch {
	case req.dstFS != nil:
	case req.HTTPWriter != nil:
		req.dstFS = &httpFS{
			writer:   req.HTTPWriter,
			fileName: req.Destination.Path,
			fileSize: req.Size,
		}
	case req.Destination.Addr != nil:
		sshClient, err := req.DialHost(ctx, req.Destination.Login, req.Destination.Addr.String())
		if err != nil {
			return trace.Wrap(err)
		}
		req.dstFS, err = OpenRemoteFilesystem(ctx, sshClient, req.ModeratedSessionID)
		if err != nil {
			return trace.Wrap(err)
		}
		expandedPath, err := ExpandHomeDir(req.Destination.Path)
		if err != nil {
			return trace.Wrap(err)
		}
		req.Destination.Path = expandedPath
	default:
		req.dstFS = localFS{}
	}
	defer req.dstFS.Close()

	return trace.Wrap(transfer(ctx, req))
}

// transfer performs file transfers
func transfer(ctx context.Context, req *FileTransferRequest) error {
	// get info of source files and ensure appropriate options were passed
	matchedPaths := make([]string, 0, len(req.Sources.Paths))
	fileInfos := make([]os.FileInfo, 0, len(req.Sources.Paths))
	for _, srcPath := range req.Sources.Paths {
		// This source path may or may not contain a glob pattern, but
		// try and glob just in case. It is also possible the user
		// specified a file path containing glob pattern characters but
		// means the literal path without globbing, in which case we'll
		// use the raw source path as the sole match below.
		matches, err := req.srcFS.Glob(srcPath)
		if err != nil {
			return trace.Wrap(err, "error matching glob pattern %q", srcPath)
		}
		if len(matches) == 0 {
			matches = []string{srcPath}
		}

		// clean match paths to ensure they are separated by backslashes, as
		// SFTP requires that
		for i := range matches {
			matches[i] = path.Clean(matches[i])
		}
		matchedPaths = append(matchedPaths, matches...)

		for _, match := range matches {
			fi, err := req.srcFS.Stat(match)
			if err != nil {
				return trace.Wrap(err, "could not access %s path %q", req.srcFS.Type(), match)
			}
			if fi.IsDir() && !req.Recursive {
				// Note: Using an error constructor included in lib/client.IsErrorResolvableWithRelogin,
				// e.g. BadParameter, will lead to relogin attempt and a completely obscure error message.
				return trace.Wrap(&NonRecursiveDirectoryTransferError{Path: match})
			}
			fileInfos = append(fileInfos, fi)
		}
	}

	// validate destination path and create it if necessary
	var dstIsDir bool
	req.Destination.Path = path.Clean(req.Destination.Path)
	dstInfo, err := req.dstFS.Stat(req.Destination.Path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return trace.NotFound("error accessing %s path %q: %v", req.dstFS.Type(), req.Destination.Path, err)
		}
		// if there are multiple source paths and the destination path
		// doesn't exist, create it as a directory
		if len(matchedPaths) > 1 {
			if err := req.dstFS.Mkdir(req.Destination.Path); err != nil {
				return trace.Errorf("error creating %s directory %q: %w", req.dstFS.Type(), req.Destination.Path, err)
			}
			if err := req.dstFS.Chmod(req.Destination.Path, defaults.DirectoryPermissions); err != nil {
				return trace.Errorf("error setting permissions of %s directory %q: %w", req.dstFS.Type(), req.Destination.Path, err)
			}
			dstIsDir = true
		}
	} else if len(matchedPaths) > 1 && !dstInfo.IsDir() {
		// if there are multiple source paths, ensure the destination path
		// is a directory
		if len(matchedPaths) != len(req.Sources.Paths) {
			return trace.BadParameter("%s file %q is not a directory, but multiple source files were matched by a glob pattern",
				req.dstFS.Type(),
				req.Destination.Path,
			)
		} else {
			return trace.BadParameter("%s file %q is not a directory, but multiple source files were specified",
				req.dstFS.Type(),
				req.Destination.Path,
			)
		}
	} else if dstInfo.IsDir() {
		dstIsDir = true
	}

	for i, fi := range fileInfos {
		dstPath := req.Destination.Path
		if dstIsDir || fi.IsDir() {
			dstPath = path.Join(dstPath, fi.Name())
		}

		if fi.IsDir() {
			if err := transferDir(ctx, req, dstPath, matchedPaths[i], fi, nil); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := transferFile(ctx, req, dstPath, matchedPaths[i], fi); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// transferDir transfers a directory
func transferDir(ctx context.Context, req *FileTransferRequest, dstPath, srcPath string, srcFileInfo os.FileInfo, visited map[string]struct{}) error {
	if visited == nil {
		visited = make(map[string]struct{})
	}
	realSrcPath, err := req.srcFS.RealPath(srcPath)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, ok := visited[realSrcPath]; ok {
		req.Log.DebugContext(ctx, "symlink loop detected, directory will be skipped", "link", srcPath, "target", realSrcPath)
		return nil
	}
	visited[realSrcPath] = struct{}{}
	req.Log.DebugContext(ctx, "transferring contents of directory",
		"source_fs", req.srcFS.Type(),
		"source_path", srcPath,
		"dest_fs", req.dstFS.Type(),
		"dest_path", dstPath,
	)

	err = req.dstFS.Mkdir(dstPath)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return trace.Errorf("error creating %s directory %q: %w", req.dstFS.Type(), dstPath, err)
	}
	if err := req.dstFS.Chmod(dstPath, srcFileInfo.Mode()); err != nil {
		return trace.Errorf("error setting permissions of %s directory %q: %w", req.dstFS.Type(), dstPath, err)
	}

	infos, err := req.srcFS.ReadDir(srcPath)
	if err != nil {
		return trace.Errorf("error reading %s directory %q: %w", req.srcFS.Type(), srcPath, err)
	}

	for _, info := range infos {
		dstSubPath := path.Join(dstPath, info.Name())
		lSubPath := path.Join(srcPath, info.Name())

		if info.IsDir() {
			if err := transferDir(ctx, req, dstSubPath, lSubPath, info, visited); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := transferFile(ctx, req, dstSubPath, lSubPath, info); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// set modification and access times last so creating sub dirs/files
	// doesn't update the times
	if req.PreserveAttrs {
		err := req.dstFS.Chtimes(dstPath, getAtime(srcFileInfo), srcFileInfo.ModTime())
		if err != nil {
			return trace.Errorf("error changing times of %s directory %q: %w", req.dstFS.Type(), dstPath, err)
		}
	}

	return nil
}

// transferFile transfers a file
func transferFile(ctx context.Context, req *FileTransferRequest, dstPath, srcPath string, srcFileInfo os.FileInfo) error {
	req.Log.DebugContext(ctx, "transferring file",
		"source_fs", req.srcFS.Type(),
		"source_file", srcPath,
		"dest_fs", req.dstFS.Type(),
		"dest_file", dstPath,
	)

	srcFile, err := req.srcFS.Open(srcPath)
	if err != nil {
		return trace.Errorf("error opening %s file %q: %w", req.srcFS.Type(), srcPath, err)
	}
	defer srcFile.Close()

	dstFile, err := req.dstFS.Create(dstPath, srcFileInfo.Size())
	if err != nil {
		return trace.Errorf("error creating %s file %q: %w", req.dstFS.Type(), dstPath, err)
	}
	defer dstFile.Close()

	if err := req.dstFS.Chmod(dstPath, srcFileInfo.Mode()); err != nil {
		return trace.Errorf("error setting permissions of %s file %q: %w", req.dstFS.Type(), dstPath, err)
	}

	var progressBar io.ReadWriter
	if req.ProgressStream != nil {
		progressBar = req.ProgressStream(srcFileInfo)
	}

	reader, writer := prepareStreams(ctx, srcFile, dstFile, progressBar)
	if err := assertStreamsType(reader, writer); err != nil {
		return trace.Wrap(err)
	}

	n, err := io.Copy(writer, reader)
	if err != nil {
		return trace.Errorf("error copying %s file %q to %s file %q: %w",
			req.srcFS.Type(),
			srcPath,
			req.dstFS.Type(),
			dstPath,
			err,
		)
	}
	if n < srcFileInfo.Size() {
		return trace.Errorf("error copying %s file %q to %s file %q: short write: wrote %d bytes, expected to write %d bytes",
			req.srcFS.Type(),
			srcPath,
			req.dstFS.Type(),
			dstPath,
			n,
			srcFileInfo.Size(),
		)
	}

	if req.PreserveAttrs {
		err := req.dstFS.Chtimes(dstPath, getAtime(srcFileInfo), srcFileInfo.ModTime())
		if err != nil {
			return trace.Errorf("error changing times of %s file %q: %w", req.dstFS.Type(), dstPath, err)
		}
	}

	return nil
}

// assertStreamsType checks if reader or writer implements correct interface to utilize concurrent SFTP streams.
func assertStreamsType(reader io.Reader, writer io.Writer) error {
	_, okReader := reader.(io.WriterTo)
	if okReader {
		_, okStat := reader.(interface{ Stat() (os.FileInfo, error) })
		if !okStat {
			return trace.Errorf("sftp read stream must implement Stat() method")
		}

		return nil
	}

	_, okWriter := writer.(io.ReaderFrom)
	if !okWriter && !okReader {
		return trace.Errorf("reader and writer are not implementing concurrent interfaces %T %T", reader, writer)
	}

	return nil
}

// prepareStreams adds passed context to the local stream and progress bar if provided.
func prepareStreams(ctx context.Context, srcFile fs.File, dstFile io.WriteCloser, progressBar io.ReadWriter) (io.Reader, io.Writer) {
	var reader io.Reader = srcFile
	var writer io.Writer = dstFile

	if _, ok := reader.(*sftp.File); ok {
		if progressBar != nil {
			writer = io.MultiWriter(dstFile, progressBar)
		} else {
			writer = dstFile
		}

		writer = &cancelWriter{
			ctx:    ctx,
			stream: writer,
		}
	} else {
		streams := make([]io.Reader, 0, 1)

		if progressBar != nil {
			streams = append(streams, progressBar)
		}

		reader = &fileStreamReader{
			ctx:     ctx,
			streams: streams,
			file:    srcFile,
		}
	}

	return reader, writer
}

func getAtime(fi os.FileInfo) time.Time {
	s := fi.Sys()
	if s == nil {
		return time.Time{}
	}

	if sftpfi, ok := fi.Sys().(*sftp.FileStat); ok {
		return time.Unix(int64(sftpfi.Atime), 0)
	}

	return scp.GetAtime(fi)
}

// unboundedProgressBar is a wrapper for a progress bar that increases its max
// value when its internal count gets too big, instead of failing.
type unboundedProgressBar struct {
	pb *progressbar.ProgressBar
}

func (u *unboundedProgressBar) checkMax(n int) {
	state := u.pb.State()
	newNum := state.CurrentNum + int64(n)
	if newNum > state.Max {
		u.pb.ChangeMax64(newNum)
	}
}

func (u *unboundedProgressBar) Read(p []byte) (int, error) {
	u.checkMax(len(p))
	return u.pb.Read(p)
}

func (u *unboundedProgressBar) Write(p []byte) (int, error) {
	u.checkMax(len(p))
	return u.pb.Write(p)
}

// newProgressBar returns a new progress bar that writes to writer.
func newProgressBar(size int64, desc string, writer io.Writer) *unboundedProgressBar {
	// this is necessary because progressbar.DefaultBytes doesn't allow
	// the caller to specify a writer
	return &unboundedProgressBar{pb: progressbar.NewOptions64(
		size,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWriter(writer),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(writer, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)}
}
