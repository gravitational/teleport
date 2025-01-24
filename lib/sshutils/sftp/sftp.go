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
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils/scp"
)

// Options control aspects of a file transfer
type Options struct {
	// Recursive indicates recursive file transfer
	Recursive bool
	// PreserveAttrs preserves access and modification times
	// from the original file
	PreserveAttrs bool
	// Quiet indicates whether progress should be displayed.
	Quiet bool
	// ProgressWriter is used to write the progress output.
	ProgressWriter io.Writer
}

// Config describes the settings of a file transfer
type Config struct {
	srcPaths []string
	dstPath  string
	srcFS    FileSystem
	dstFS    FileSystem
	opts     Options

	// ProgressStream is a callback to return a read/writer for printing the progress
	// (used only on the client)
	ProgressStream func(fileInfo os.FileInfo) io.ReadWriter
	// Log optionally specifies the logger
	Log log.FieldLogger
}

// FileSystem describes file operations to be done either locally or over SFTP
type FileSystem interface {
	// Type returns whether the filesystem is "local" or "remote"
	Type() string
	// Glob returns matching files of a glob pattern
	Glob(ctx context.Context, pattern string) ([]string, error)
	// Stat returns info about a file
	Stat(ctx context.Context, path string) (os.FileInfo, error)
	// ReadDir returns information about files contained within a directory
	ReadDir(ctx context.Context, path string) ([]os.FileInfo, error)
	// Open opens a file
	Open(ctx context.Context, path string) (fs.File, error)
	// Create creates a new file
	Create(ctx context.Context, path string, size int64) (io.WriteCloser, error)
	// Mkdir creates a directory
	Mkdir(ctx context.Context, path string) error
	// Chmod sets file permissions
	Chmod(ctx context.Context, path string, mode os.FileMode) error
	// Chtimes sets file access and modification time
	Chtimes(ctx context.Context, path string, atime, mtime time.Time) error
}

// CreateUploadConfig returns a Config ready to upload files over SFTP.
func CreateUploadConfig(src []string, dst string, opts Options) (*Config, error) {
	for _, srcPath := range src {
		if srcPath == "" {
			return nil, trace.BadParameter("source path is empty")
		}
	}
	if dst == "" {
		return nil, trace.BadParameter("destination path is empty")
	}

	c := &Config{
		srcPaths: src,
		dstPath:  dst,
		srcFS:    &localFS{},
		dstFS:    &remoteFS{},
		opts:     opts,
	}
	c.setDefaults()

	return c, nil
}

// CreateDownloadConfig returns a Config ready to download files over SFTP.
func CreateDownloadConfig(src, dst string, opts Options) (*Config, error) {
	if src == "" {
		return nil, trace.BadParameter("source path is empty")
	}
	if dst == "" {
		return nil, trace.BadParameter("destination path is empty")
	}

	c := &Config{
		srcPaths: []string{src},
		dstPath:  dst,
		srcFS:    &remoteFS{},
		dstFS:    &localFS{},
		opts:     opts,
	}
	c.setDefaults()

	return c, nil
}

// HTTPTransferRequest describes file transfer request over HTTP.
type HTTPTransferRequest struct {
	// Src is the source file name
	Src string
	// Dst is the destination file name
	Dst string
	// HTTPRequest is where the source file will be read from for
	// file upload transfers
	HTTPRequest *http.Request
	// HTTPResponse is where the destination file will be written to for
	// file download transfers
	HTTPResponse http.ResponseWriter
}

// CreateHTTPUploadConfig returns a Config ready to upload a file from
// a HTTP request over SFTP.
func CreateHTTPUploadConfig(req HTTPTransferRequest) (*Config, error) {
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

	c := &Config{
		srcPaths: []string{req.Src},
		dstPath:  req.Dst,
		srcFS: &httpFS{
			reader:   req.HTTPRequest.Body,
			fileName: req.Src,
			fileSize: fileSize,
		},
		dstFS: &remoteFS{},
	}
	c.setDefaults()

	return c, nil
}

// CreateHTTPDownloadConfig returns a Config ready to download a file
// from over SFTP and write it to a HTTP response.
func CreateHTTPDownloadConfig(req HTTPTransferRequest) (*Config, error) {
	if err := req.checkDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.HTTPResponse == nil {
		return nil, trace.BadParameter("HTTP response is empty")
	}

	c := &Config{
		srcPaths: []string{req.Src},
		dstPath:  req.Dst,
		srcFS:    &remoteFS{},
		dstFS: &httpFS{
			writer:   req.HTTPResponse,
			fileName: req.Dst,
		},
	}
	c.setDefaults()

	return c, nil
}

func (h HTTPTransferRequest) checkDefaults() error {
	if h.Src == "" {
		return trace.BadParameter("source path is empty")
	}
	if h.Dst == "" {
		return trace.BadParameter("destination path is empty")
	}
	return nil
}

// setDefaults sets default values
func (c *Config) setDefaults() {
	logger := c.Log
	if logger == nil {
		logger = log.StandardLogger()
	}
	c.Log = logger.WithFields(log.Fields{
		teleport.ComponentKey: "SFTP",
		teleport.ComponentFields: log.Fields{
			"SrcPaths":      c.srcPaths,
			"DstPath":       c.dstPath,
			"Recursive":     c.opts.Recursive,
			"PreserveAttrs": c.opts.PreserveAttrs,
		},
	})

	if !c.opts.Quiet {
		c.ProgressStream = func(fileInfo os.FileInfo) io.ReadWriter {
			return NewProgressBar(fileInfo.Size(), fileInfo.Name(), cmp.Or(c.opts.ProgressWriter, io.Writer(os.Stdout)))
		}
	}
}

// TransferFiles transfers files from the configured source paths to the
// configured destination path over SFTP or HTTP depending on the Config.
func (c *Config) TransferFiles(ctx context.Context, sshClient *ssh.Client) error {
	s, err := sshClient.NewSession()
	if err != nil {
		return trace.Wrap(err)
	}
	defer s.Close()

	// File transfers in a moderated session require this variable
	// to check for approval on the ssh server
	if moderatedSessionID, ok := ctx.Value(ModeratedSessionID).(string); ok {
		s.Setenv(string(ModeratedSessionID), moderatedSessionID)
	}

	pe, err := s.StderrPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.RequestSubsystem(teleport.SFTPSubsystem); err != nil {
		// If the subsystem request failed and a generic error is
		// returned, return the session's stderr as the error if it's
		// non-empty, as the session's stderr may have a more useful
		// error message. String comparison is only used here because
		// the error is not exported.
		if strings.Contains(err.Error(), "ssh: subsystem request failed") {
			var sb strings.Builder
			if n, _ := io.Copy(&sb, pe); n > 0 {
				return trace.Wrap(errors.New(sb.String()))
			}
		}
		return trace.Wrap(err)
	}
	pw, err := s.StdinPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	pr, err := s.StdoutPipe()
	if err != nil {
		return trace.Wrap(err)
	}
	sftpClient, err := sftp.NewClientPipe(pr, pw,
		// Use concurrent stream to speed up transfer on slow networks as described in
		// https://github.com/gravitational/teleport/issues/20579
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := c.initFS(sshClient, sftpClient); err != nil {
		return trace.Wrap(err)
	}

	transferErr := c.transfer(ctx)
	closeErr := sftpClient.Close()
	if transferErr != nil {
		return trace.Wrap(transferErr)
	}

	return trace.Wrap(closeErr)
}

// initFS ensures the source and destination filesystems are ready to transfer
func (c *Config) initFS(sshClient *ssh.Client, client *sftp.Client) error {
	var haveRemoteFS bool
	srcFS, srcOK := c.srcFS.(*remoteFS)
	if srcOK {
		srcFS.c = client
		haveRemoteFS = true
	}
	dstFS, dstOK := c.dstFS.(*remoteFS)
	if dstOK {
		dstFS.c = client
		haveRemoteFS = true
	}
	// this will only happen in tests
	if !haveRemoteFS {
		return nil
	}

	return trace.Wrap(c.expandPaths(srcOK, dstOK))
}

func (c *Config) expandPaths(srcIsRemote, dstIsRemote bool) (err error) {
	if srcIsRemote {
		for i, srcPath := range c.srcPaths {
			c.srcPaths[i], err = expandPath(srcPath)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	if dstIsRemote {
		c.dstPath, err = expandPath(c.dstPath)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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
func (c *Config) transfer(ctx context.Context) error {
	// get info of source files and ensure appropriate options were passed
	matchedPaths := make([]string, 0, len(c.srcPaths))
	fileInfos := make([]os.FileInfo, 0, len(c.srcPaths))
	for _, srcPath := range c.srcPaths {
		// This source path may or may not contain a glob pattern, but
		// try and glob just in case. It is also possible the user
		// specified a file path containing glob pattern characters but
		// means the literal path without globbing, in which case we'll
		// use the raw source path as the sole match below.
		matches, err := c.srcFS.Glob(ctx, srcPath)
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
			fi, err := c.srcFS.Stat(ctx, match)
			if err != nil {
				return trace.Wrap(err, "could not access %s path %q", c.srcFS.Type(), match)
			}
			if fi.IsDir() && !c.opts.Recursive {
				// Note: Using an error constructor included in lib/client.IsErrorResolvableWithRelogin,
				// e.g. BadParameter, will lead to relogin attempt and a completely obscure error message.
				return trace.Wrap(&NonRecursiveDirectoryTransferError{Path: match})
			}
			fileInfos = append(fileInfos, fi)
		}
	}

	// validate destination path and create it if necessary
	var dstIsDir bool
	c.dstPath = path.Clean(c.dstPath)
	dstInfo, err := c.dstFS.Stat(ctx, c.dstPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return trace.NotFound("error accessing %s path %q: %v", c.dstFS.Type(), c.dstPath, err)
		}
		// if there are multiple source paths and the destination path
		// doesn't exist, create it as a directory
		if len(matchedPaths) > 1 {
			if err := c.dstFS.Mkdir(ctx, c.dstPath); err != nil {
				return trace.Errorf("error creating %s directory %q: %w", c.dstFS.Type(), c.dstPath, err)
			}
			if err := c.dstFS.Chmod(ctx, c.dstPath, defaults.DirectoryPermissions); err != nil {
				return trace.Errorf("error setting permissions of %s directory %q: %w", c.dstFS.Type(), c.dstPath, err)
			}
			dstIsDir = true
		}
	} else if len(matchedPaths) > 1 && !dstInfo.IsDir() {
		// if there are multiple source paths, ensure the destination path
		// is a directory
		if len(matchedPaths) != len(c.srcPaths) {
			return trace.BadParameter("%s file %q is not a directory, but multiple source files were matched by a glob pattern",
				c.dstFS.Type(),
				c.dstPath,
			)
		} else {
			return trace.BadParameter("%s file %q is not a directory, but multiple source files were specified",
				c.dstFS.Type(),
				c.dstPath,
			)
		}
	} else if dstInfo.IsDir() {
		dstIsDir = true
	}

	for i, fi := range fileInfos {
		dstPath := c.dstPath
		if dstIsDir || fi.IsDir() {
			dstPath = path.Join(dstPath, fi.Name())
		}

		if fi.IsDir() {
			if err := c.transferDir(ctx, dstPath, matchedPaths[i], fi); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := c.transferFile(ctx, dstPath, matchedPaths[i], fi); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// transferDir transfers a directory
func (c *Config) transferDir(ctx context.Context, dstPath, srcPath string, srcFileInfo os.FileInfo) error {
	c.Log.Debugf("copying %s dir %q to %s dir %q", c.srcFS.Type(), srcPath, c.dstFS.Type(), dstPath)

	err := c.dstFS.Mkdir(ctx, dstPath)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return trace.Errorf("error creating %s directory %q: %w", c.dstFS.Type(), dstPath, err)
	}
	if err := c.dstFS.Chmod(ctx, dstPath, srcFileInfo.Mode()); err != nil {
		return trace.Errorf("error setting permissions of %s directory %q: %w", c.dstFS.Type(), dstPath, err)
	}

	infos, err := c.srcFS.ReadDir(ctx, srcPath)
	if err != nil {
		return trace.Errorf("error reading %s directory %q: %w", c.srcFS.Type(), srcPath, err)
	}

	for _, info := range infos {
		dstSubPath := path.Join(dstPath, info.Name())
		lSubPath := path.Join(srcPath, info.Name())

		if info.IsDir() {
			if err := c.transferDir(ctx, dstSubPath, lSubPath, info); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := c.transferFile(ctx, dstSubPath, lSubPath, info); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// set modification and access times last so creating sub dirs/files
	// doesn't update the times
	if c.opts.PreserveAttrs {
		err := c.dstFS.Chtimes(ctx, dstPath, getAtime(srcFileInfo), srcFileInfo.ModTime())
		if err != nil {
			return trace.Errorf("error changing times of %s directory %q: %w", c.dstFS.Type(), dstPath, err)
		}
	}

	return nil
}

// transferFile transfers a file
func (c *Config) transferFile(ctx context.Context, dstPath, srcPath string, srcFileInfo os.FileInfo) error {
	c.Log.Debugf("copying %s file %q to %s file %q", c.srcFS.Type(), srcPath, c.dstFS.Type(), dstPath)

	srcFile, err := c.srcFS.Open(ctx, srcPath)
	if err != nil {
		return trace.Errorf("error opening %s file %q: %w", c.srcFS.Type(), srcPath, err)
	}
	defer srcFile.Close()

	dstFile, err := c.dstFS.Create(ctx, dstPath, srcFileInfo.Size())
	if err != nil {
		return trace.Errorf("error creating %s file %q: %w", c.dstFS.Type(), dstPath, err)
	}
	defer dstFile.Close()

	if err := c.dstFS.Chmod(ctx, dstPath, srcFileInfo.Mode()); err != nil {
		return trace.Errorf("error setting permissions of %s file %q: %w", c.dstFS.Type(), dstPath, err)
	}

	var progressBar io.ReadWriter
	if c.ProgressStream != nil {
		progressBar = c.ProgressStream(srcFileInfo)
	}

	reader, writer := prepareStreams(ctx, srcFile, dstFile, progressBar)
	if err := assertStreamsType(reader, writer); err != nil {
		return trace.Wrap(err)
	}

	n, err := io.Copy(writer, reader)
	if err != nil {
		return trace.Errorf("error copying %s file %q to %s file %q: %w",
			c.srcFS.Type(),
			srcPath,
			c.dstFS.Type(),
			dstPath,
			err,
		)
	}
	if n != srcFileInfo.Size() {
		return trace.Errorf("error copying %s file %q to %s file %q: short write: wrote %d bytes, expected to write %d bytes",
			c.srcFS.Type(),
			srcPath,
			c.dstFS.Type(),
			dstPath,
			n,
			srcFileInfo.Size(),
		)
	}

	if c.opts.PreserveAttrs {
		err := c.dstFS.Chtimes(ctx, dstPath, getAtime(srcFileInfo), srcFileInfo.ModTime())
		if err != nil {
			return trace.Errorf("error changing times of %s file %q: %w", c.dstFS.Type(), dstPath, err)
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
