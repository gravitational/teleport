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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils/scp"
)

// SFTP request methods.
const (
	// MethodGet opens a file for reading.
	MethodGet = "Get"
	// MethodPut opens a file for writing.
	MethodPut = "Put"
	// MethodOpen opens a file.
	MethodOpen = "Open"
	// MethodSetStat sets a file's stats.
	MethodSetStat = "Setstat"
	// MethodRename renames a file.
	MethodRename = "Rename"
	// MethodRmdir removes a directory.
	MethodRmdir = "Rmdir"
	// MethodMkdir creates a directory.
	MethodMkdir = "Mkdir"
	// MethodLink creates a hard link.
	MethodLink = "Link"
	// MethodSymlink creates a symbolic link.
	MethodSymlink = "Symlink"
	// MethodRemove deletes a file.
	MethodRemove = "Remove"
	// MethodList lists directory entries.
	MethodList = "List"
	// MethodStat gets a directory entry's stat info.
	MethodStat = "Stat"
	// MethodLstat gets a directory entry's stat info, without following symbolic links.
	MethodLstat = "Lstat"
	// MethodReadlink gets the target of a symbolic link.
	MethodReadlink = "Readlink"
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
			return NewProgressBar(fileInfo.Size(), fileInfo.Name(), req.ProgressWriter)
		}
	}
	return nil
}

// File is the file interface required for [FileSystem].
type File interface {
	sftp.WriterAtReaderAt
	io.ReadWriteCloser
	// Name returns the name of the file.
	Name() string
	// Stat returns the files stat info.
	Stat() (fs.FileInfo, error)
}

// FileSystem describes file operations to be done either locally or over SFTP.
//
// Note: errors returned by a FileSystem should not be `trace.Wrap()`ed so the
// sftp package can parse os errors.
type FileSystem interface {
	io.Closer
	// Type returns whether the filesystem is "local" or "remote".
	Type() string
	// Glob returns matching files of a glob pattern.
	Glob(pattern string) ([]string, error)
	// Stat returns info about a file.
	Stat(path string) (os.FileInfo, error)
	// ReadDir returns information about files contained within a directory.
	ReadDir(path string) ([]os.FileInfo, error)
	// Open opens a file for reading.
	Open(path string) (File, error)
	// Create creates a new file for writing.
	Create(path string, size int64) (File, error)
	// Mkdir creates a directory.
	Mkdir(path string) error
	// Chmod sets file permissions.
	Chmod(path string, mode os.FileMode) error
	// Chtimes sets file access and modification time.
	Chtimes(path string, atime, mtime time.Time) error
	// OpenFile opens a file with the given flags.
	OpenFile(path string, flags int) (File, error)
	// Rename renames a file.
	Rename(oldpath, newpath string) error
	// Lstat returns info about a file or symlink.
	Lstat(name string) (os.FileInfo, error)
	// RemoveAll recursively removes a file or directory.
	RemoveAll(path string) error
	// Link creates a new link.
	Link(oldname, newname string) error
	// Symlink creates a new symlink.
	Symlink(oldname, newname string) error
	// Remove removes a file or (empty) directory.
	Remove(name string) error
	// Chown changes a file's owner and/or group.
	Chown(name string, uid, gid int) error
	// Truncate truncates a file's contents.
	Truncate(name string, size int64) error
	// Readlink gets the destination for a symlink.
	Readlink(name string) (string, error)
	// Getwd gets the current working directory.
	Getwd() (string, error)
	// RealPath canonicalizes a path name, including resolving ".." and
	// following symlinks.
	RealPath(path string) (string, error)
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
			expandedPath, err := expandPath(srcPath)
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
		expandedPath, err := expandPath(req.Destination.Path)
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

// PathExpansionError is an [error] indicating that
// path expansion was rejected.
type PathExpansionError struct {
	path string
}

func (p PathExpansionError) Error() string {
	return fmt.Sprintf("expanding remote ~user paths is not supported, specify an absolute path instead of %q", p.path)
}

func expandPath(pathStr string) (string, error) {
	pfxLen, ok := homeDirPrefixLen(pathStr)
	if !ok {
		return pathStr, nil
	}

	// Removing the home dir prefix would mean returning an empty string,
	// which is supported by SFTP but won't be as clear in logs or audit
	// events. Since the SFTP server will be rooted at the user's home
	// directory, "." and "" are equivalent in this context.
	if pathStr == "~" {
		return ".", nil
	}
	if pfxLen == 1 && len(pathStr) > 1 {
		return "", trace.Wrap(PathExpansionError{path: pathStr})
	}

	// if an SFTP path is not absolute, it is assumed to start at the user's
	// home directory so just strip the prefix and let the SFTP server
	// figure out the correct remote path
	return pathStr[pfxLen:], nil
}

// homeDirPrefixLen returns the length of a set of characters that
// indicates the user wants the path to begin with a user's home
// directory and a bool that indicates whether such a prefix exists.
func homeDirPrefixLen(path string) (int, bool) {
	if strings.HasPrefix(path, "~/") {
		return 2, true
	}
	// allow '~\' or '~/' on Windows since '\' is the canonical path
	// separator but some users may use '/' instead
	if runtime.GOOS == "windows" && strings.HasPrefix(path, `~\`) {
		return 2, true
	}

	if len(path) >= 1 && path[0] == '~' {
		return 1, true
	}

	return -1, false
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
	if n != srcFileInfo.Size() {
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

// NewProgressBar returns a new progress bar that writes to writer.
func NewProgressBar(size int64, desc string, writer io.Writer) *progressbar.ProgressBar {
	// this is necessary because progressbar.DefaultBytes doesn't allow
	// the caller to specify a writer
	return progressbar.NewOptions64(
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
	)
}

// NonRecursiveDirectoryTransferError is returned when an attempt is made
// to download a directory without providing the recursive option.
// It's used to distinguish this specific situation in clients which
// do not support the recursive option.
type NonRecursiveDirectoryTransferError struct {
	Path string
}

func (n *NonRecursiveDirectoryTransferError) Error() string {
	return fmt.Sprintf("%q is a directory, but the recursive option was not passed", n.Path)
}

func setstat(req *sftp.Request, fs FileSystem) error {
	attrFlags := req.AttrFlags()
	attrs := req.Attributes()

	if attrFlags.Acmodtime {
		atime := time.Unix(int64(attrs.Atime), 0)
		mtime := time.Unix(int64(attrs.Mtime), 0)

		err := fs.Chtimes(req.Filepath, atime, mtime)
		if err != nil {
			return err
		}
	}
	if attrFlags.Permissions {
		err := fs.Chmod(req.Filepath, attrs.FileMode())
		if err != nil {
			return err
		}
	}
	if attrFlags.UidGid {
		err := fs.Chown(req.Filepath, int(attrs.UID), int(attrs.GID))
		if err != nil {
			return err
		}
	}
	if attrFlags.Size {
		err := fs.Truncate(req.Filepath, int64(attrs.Size))
		if err != nil {
			return err
		}
	}

	return nil
}

// HandleFilecmd handles file command requests. If filesys is nil, the local
// filesystem will be used.
func HandleFilecmd(req *sftp.Request, filesys FileSystem) error {
	if filesys == nil {
		filesys = localFS{}
	}
	switch req.Method {
	case MethodSetStat:
		return setstat(req, filesys)
	case MethodRename:
		if req.Target == "" {
			return os.ErrInvalid
		}
		return filesys.Rename(req.Filepath, req.Target)
	case MethodRmdir:
		fi, err := filesys.Lstat(req.Filepath)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return fmt.Errorf("%q is not a directory", req.Filepath)
		}
		return filesys.RemoveAll(req.Filepath)
	case MethodMkdir:
		return filesys.Mkdir(req.Filepath)
	case MethodLink:
		if req.Target == "" {
			return os.ErrInvalid
		}
		return filesys.Link(req.Target, req.Filepath)
	case MethodSymlink:
		if req.Target == "" {
			return os.ErrInvalid
		}
		return filesys.Symlink(req.Target, req.Filepath)
	case MethodRemove:
		fi, err := filesys.Lstat(req.Filepath)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return fmt.Errorf("%q is a directory", req.Filepath)
		}
		return filesys.Remove(req.Filepath)
	default:
		return sftp.ErrSSHFxOpUnsupported
	}
}

// listerAt satisfies [sftp.listerAt].
type listerAt []fs.FileInfo

func (l listerAt) ListAt(ls []fs.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(ls, l[offset:])
	if n < len(ls) {
		return n, io.EOF
	}

	return n, nil
}

// fileName satisfies [fs.FileInfo] but only knows a file's name. This
// is necessary when handling 'readlink' requests in sftpHandler.FileList,
// as only the file's name is known after a readlink call.
type fileName string

func (f fileName) Name() string {
	return string(f)
}

func (f fileName) Size() int64 {
	return 0
}

func (f fileName) Mode() fs.FileMode {
	return 0
}

func (f fileName) ModTime() time.Time {
	return time.Time{}
}

func (f fileName) IsDir() bool {
	return false
}

func (f fileName) Sys() any {
	return nil
}

// HandleFilelist handles file list requests. If filesys is nil, the local
// filesystem will be used.
func HandleFilelist(req *sftp.Request, filesys FileSystem) (sftp.ListerAt, error) {
	if filesys == nil {
		filesys = localFS{}
	}
	switch req.Method {
	case MethodList:
		entries, err := filesys.ReadDir(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerAt(entries), nil
	case MethodStat:
		fi, err := filesys.Stat(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerAt{fi}, nil
	case MethodReadlink:
		dst, err := filesys.Readlink(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerAt{fileName(dst)}, nil
	default:
		return nil, sftp.ErrSSHFxOpUnsupported
	}
}
