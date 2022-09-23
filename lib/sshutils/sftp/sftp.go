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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/trace"

	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// Options control aspects of a file transfer
type Options struct {
	// Recursive indicates recursive file transfer
	Recursive bool
	// PreserveAttrs preserves access and modification times
	// from the original file
	PreserveAttrs bool
}

// Config describes the settings of a file transfer
type Config struct {
	srcPaths []string
	dstPath  string
	srcFS    FileSystem
	dstFS    FileSystem
	opts     Options

	// ProgressWriter is a writer for printing the progress
	// (used only on the client)
	ProgressWriter io.Writer
	// Log optionally specifies the logger
	Log log.FieldLogger
}

// FileSystem describes file operations to be done either locally or over SFTP
type FileSystem interface {
	// SetContext sets the context for the filesystem used for cancellation
	SetContext(ctx context.Context)
	// Type returns whether the filesystem is "local" or "remote"
	Type() string
	// Stat returns info about a file
	Stat(path string) (os.FileInfo, error)
	// ReadDir returns information about files contained within a directory
	ReadDir(path string) ([]os.FileInfo, error)
	// Open opens a file
	Open(path string) (io.ReadCloser, error)
	// Create creates a new file
	Create(path string, length uint64) (io.WriteCloser, error)
	// MkDir creates a directory
	// sftp.Client.Mkdir does not take an os.FileMode, so this can't either
	Mkdir(path string) error
	// Chmod sets file permissions
	Chmod(path string, mode os.FileMode) error
	// Chtimes sets file access and modification time
	Chtimes(path string, atime, mtime time.Time) error
}

// CreateUploadConfig returns a Config ready to upload files
func CreateUploadConfig(src []string, dst string, opts Options) *Config {
	c := &Config{
		srcPaths: src,
		dstPath:  dst,
		srcFS:    &localFS{},
		dstFS:    &remoteFS{},
		opts:     opts,
	}
	c.setDefaults()

	return c
}

// CreateDownloadConfig returns a Config ready to download files
func CreateDownloadConfig(src, dst string, opts Options) *Config {
	c := &Config{
		srcPaths: []string{src},
		dstPath:  dst,
		srcFS:    &remoteFS{},
		dstFS:    &localFS{},
		opts:     opts,
	}
	c.setDefaults()

	return c
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
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return trace.Wrap(err)
	}
	c.initFS(ctx, sftpClient)

	transferErr := c.transfer(ctx)
	closeErr := sftpClient.Close()
	if transferErr != nil {
		return trace.Wrap(transferErr)
	}
	if closeErr != nil {
		return trace.Wrap(closeErr)
	}
	return nil
}

// initFS ensures the source and destination filesystems are ready to transfer
func (c *Config) initFS(ctx context.Context, client *sftp.Client) {
	srcFS, ok := c.srcFS.(*remoteFS)
	if ok {
		srcFS.c = client
	}
	dstFS, ok := c.dstFS.(*remoteFS)
	if ok {
		dstFS.c = client
	}
	c.srcFS.SetContext(ctx)
	c.dstFS.SetContext(ctx)
}

// transfer preforms file transfers
func (c *Config) transfer(ctx context.Context) error {
	// if there are multiple source paths, ensure the destination path
	// is a directory
	var dirMode bool
	if len(c.srcPaths) > 1 {
		fi, err := c.dstFS.Stat(c.dstPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return trace.Errorf("error accessing %s path %q: %v", c.dstFS.Type(), c.dstPath, err)
			}
			if err := c.dstFS.Mkdir(c.dstPath); err != nil {
				return trace.Wrap(err)
			}
		} else if !fi.IsDir() {
			return trace.BadParameter("%s file %q is not a directory, but multiple source files were specified",
				c.dstFS.Type(),
				c.dstPath,
			)
		}
		dirMode = true
	}

	// get info of source files and ensure appropriate options were passed
	fileInfos := make([]os.FileInfo, len(c.srcPaths))
	for i := range c.srcPaths {
		fi, err := c.srcFS.Stat(c.srcPaths[i])
		if err != nil {
			return trace.Errorf("could not access %s path %q: %v", c.srcFS.Type(), c.srcPaths[i], err)
		}
		if fi.IsDir() && !c.opts.Recursive {
			// Note: using any other error constructor (e.g. BadParameter)
			// might lead to relogin attempt and a completely obscure
			// error message
			return trace.BadParameter("%q is a directory, but the recursive option was not passed", c.srcPaths[i])
		}
		fileInfos[i] = fi
	}

	for i, fi := range fileInfos {
		dstPath := c.dstPath
		if dirMode {
			dstPath = path.Join(dstPath, fi.Name())
		}

		if fi.IsDir() {
			if err := c.transferDir(ctx, dstPath, c.srcPaths[i], fi); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := c.transferFile(ctx, dstPath, c.srcPaths[i], fi); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// transferDir transfers a directory
func (c *Config) transferDir(ctx context.Context, dstPath, srcPath string, srcFileInfo os.FileInfo) error {
	// TODO: update sftp to propagate os.ErrExist
	err := c.dstFS.Mkdir(dstPath)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		return trace.Wrap(err)
	}

	if c.opts.PreserveAttrs {
		err := c.dstFS.Chtimes(dstPath, getAtime(srcFileInfo), srcFileInfo.ModTime())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err := c.dstFS.Chmod(dstPath, srcFileInfo.Mode()); err != nil {
		return trace.Wrap(err)
	}

	infos, err := c.srcFS.ReadDir(srcPath)
	if err != nil {
		return trace.Wrap(err)
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

	return nil
}

// transferFile transfers a file
func (c *Config) transferFile(ctx context.Context, dstPath, srcPath string, srcFileInfo os.FileInfo) error {
	srcFile, err := c.srcFS.Open(srcPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer srcFile.Close()

	// TODO: sftp Open flags not getting emitted
	dstFile, err := c.dstFS.Create(dstPath, uint64(srcFileInfo.Size()))
	if err != nil {
		return trace.Wrap(err)
	}
	defer dstFile.Close()

	// write to canceler first so if the context is canceled the transferring
	// can stop immediately
	var writer io.Writer
	canceler := &cancelWriter{
		ctx: ctx,
	}
	// if a progress writer was set, write file transfer progress
	if c.ProgressWriter != nil {
		progressBar := newProgressBar(srcFileInfo.Size(), srcFileInfo.Name(), c.ProgressWriter)
		writer = io.MultiWriter(canceler, dstFile, progressBar)
	} else {
		writer = io.MultiWriter(canceler, dstFile)
	}

	n, err := io.Copy(writer, srcFile)
	if err != nil {
		return trace.Wrap(err)
	}
	if n != srcFileInfo.Size() {
		return trace.Errorf("short write: written %v, expected %v", n, srcFileInfo.Size())
	}

	if c.opts.PreserveAttrs {
		err := c.dstFS.Chtimes(dstPath, getAtime(srcFileInfo), srcFileInfo.ModTime())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = c.dstFS.Chmod(dstPath, srcFileInfo.Mode())
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func getAtime(fi os.FileInfo) time.Time {
	s := fi.Sys()
	if s != nil {
		if sftpfi, ok := fi.Sys().(*sftp.FileStat); ok {
			return time.Unix(int64(sftpfi.Atime), 0)
		}
		return scp.GetAtime(fi)
	}

	return time.Time{}
}

type cancelWriter struct {
	ctx context.Context
}

func (c *cancelWriter) Write(b []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return len(b), nil
}

// newProgressBar returns a new progress bar that writes to writer.
func newProgressBar(size int64, desc string, writer io.Writer) *progressbar.ProgressBar {
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
