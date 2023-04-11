/*
Copyright 2022 Gravitational, Inc.

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

// Package sftp handles file transfers client-side via SFTP
package sftp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path" // SFTP requires UNIX-style path separators
	"runtime"
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
}

type homeDirRetriever func() (string, error)

// Config describes the settings of a file transfer
type Config struct {
	srcPaths []string
	dstPath  string
	srcFS    FileSystem
	dstFS    FileSystem
	opts     Options

	// getHomeDir returns the home directory of the remote user of the
	// SSH session
	getHomeDir homeDirRetriever

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
	Create(ctx context.Context, path string) (io.WriteCloser, error)
	// Mkdir creates a directory
	Mkdir(ctx context.Context, path string) error
	// Chmod sets file permissions
	Chmod(ctx context.Context, path string, mode os.FileMode) error
	// Chtimes sets file access and modification time
	Chtimes(ctx context.Context, path string, atime, mtime time.Time) error
}

// CreateUploadConfig returns a Config ready to upload files
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

// CreateDownloadConfig returns a Config ready to download files
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

// setDefaults sets default values
func (c *Config) setDefaults() {
	logger := c.Log
	if logger == nil {
		logger = log.StandardLogger()
	}
	c.Log = logger.WithFields(log.Fields{
		trace.Component: "SFTP",
		trace.ComponentFields: log.Fields{
			"SrcPaths":      c.srcPaths,
			"DstPath":       c.dstPath,
			"Recursive":     c.opts.Recursive,
			"PreserveAttrs": c.opts.PreserveAttrs,
		},
	})
}

// TransferFiles transfers files from the configured source paths to the
// configured destination path over SFTP
func (c *Config) TransferFiles(ctx context.Context, sshClient *ssh.Client) error {
	sftpClient, err := sftp.NewClient(sshClient,
		// Use concurrent stream to speed up transfer on slow networks as described in
		// https://github.com/gravitational/teleport/issues/20579
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true))
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

	if c.getHomeDir == nil {
		c.getHomeDir = func() (string, error) {
			return getRemoteHomeDir(sshClient)
		}
	}

	return trace.Wrap(c.expandPaths(srcOK, dstOK))
}

func (c *Config) expandPaths(srcIsRemote, dstIsRemote bool) (err error) {
	srcHomeRetriever := getLocalHomeDir
	if srcIsRemote {
		srcHomeRetriever = c.getHomeDir
	}
	for i, srcPath := range c.srcPaths {
		c.srcPaths[i], err = expandPath(srcPath, srcHomeRetriever)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	dstHomeRetriever := getLocalHomeDir
	if dstIsRemote {
		dstHomeRetriever = c.getHomeDir
	}
	c.dstPath, err = expandPath(c.dstPath, dstHomeRetriever)

	return trace.Wrap(err)
}

func getLocalHomeDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return u.HomeDir, nil
}

func expandPath(pathStr string, getHomeDir homeDirRetriever) (string, error) {
	if !needsExpansion(pathStr) {
		return pathStr, nil
	}

	homeDir, err := getHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	// this is safe because we verified that all paths are non-empty
	// in CreateUploadConfig/CreateDownloadConfig
	return path.Join(homeDir, pathStr[1:]), nil
}

// needsExpansion returns true if path is '~', '~/', or '~\' on Windows
func needsExpansion(path string) bool {
	if len(path) == 1 {
		return path == "~"
	}

	// allow '~\' or '~/' on Windows since '\' is the canonical path
	// separator but some users may use '/' instead
	if runtime.GOOS == "windows" && strings.HasPrefix(path, `~\`) {
		return true
	}
	return strings.HasPrefix(path, "~/")
}

// getRemoteHomeDir returns the home directory of the remote user of
// the SSH connection
func getRemoteHomeDir(sshClient *ssh.Client) (string, error) {
	s, err := sshClient.NewSession()
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer s.Close()
	if err := s.RequestSubsystem(teleport.GetHomeDirSubsystem); err != nil {
		return "", trace.Wrap(err)
	}
	r, err := s.StdoutPipe()
	if err != nil {
		return "", trace.Wrap(err)
	}

	var homeDirBuf bytes.Buffer
	if _, err := io.Copy(&homeDirBuf, r); err != nil {
		return "", trace.Wrap(err)
	}

	return homeDirBuf.String(), nil
}

// transfer performs file transfers
func (c *Config) transfer(ctx context.Context) error {
	// get info of source files and ensure appropriate options were passed
	matchedPaths := make([]string, 0, len(c.srcPaths))
	fileInfos := make([]os.FileInfo, 0, len(c.srcPaths))
	for _, srcPath := range c.srcPaths {
		matches, err := c.srcFS.Glob(ctx, srcPath)
		if err != nil {
			return trace.Wrap(err, "error matching glob pattern %q", srcPath)
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
				// Note: using any other error constructor than BadParameter
				// might lead to relogin attempt and a completely obscure
				// error message
				return trace.BadParameter("%q is a directory, but the recursive option was not passed", match)
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
	srcFile, err := c.srcFS.Open(ctx, srcPath)
	if err != nil {
		return trace.Errorf("error opening %s file %q: %w", c.srcFS.Type(), srcPath, err)
	}
	defer srcFile.Close()

	dstFile, err := c.dstFS.Create(ctx, dstPath)
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
			return trace.Errorf("sftp read stream must implement Sync() method")
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
