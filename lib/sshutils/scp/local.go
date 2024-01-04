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

package scp

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// localFileSystem provides API for accessing the files on
// the local file system
type localFileSystem struct {
}

// Chmod sets file permissions
func (l *localFileSystem) Chmod(path string, mode int) error {
	chmode := os.FileMode(mode & int(os.ModePerm))
	if err := os.Chmod(path, chmode); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Chtimes sets file access and modification times
func (l *localFileSystem) Chtimes(path string, atime, mtime time.Time) error {
	return trace.ConvertSystemError(os.Chtimes(path, atime, mtime))
}

// MkDir creates a directory
func (l *localFileSystem) MkDir(path string, mode int) error {
	fileMode := os.FileMode(mode & int(os.ModePerm))
	err := os.Mkdir(path, fileMode)
	if err != nil && !os.IsExist(err) {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// IsDir tells if a given path is a directory
func (l *localFileSystem) IsDir(path string) bool {
	return utils.IsDir(path)
}

// OpenFile opens a file for read operations and returns a Reader
func (l *localFileSystem) OpenFile(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

// GetFileInfo returns FileInfo for a given file path
func (l *localFileSystem) GetFileInfo(filePath string) (FileInfo, error) {
	info, err := makeFileInfo(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return info, nil
}

// CreateFile creates a new file and returns a Writer
func (l *localFileSystem) CreateFile(filePath string, length uint64) (io.WriteCloser, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

func makeFileInfo(filePath string) (FileInfo, error) {
	f, err := os.Stat(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &localFileInfo{
		filePath:   filePath,
		fileInfo:   f,
		accessTime: GetAtime(f),
	}, nil
}

// localFileInfo is implementation of FileInfo for local files
type localFileInfo struct {
	filePath   string
	fileInfo   os.FileInfo
	accessTime time.Time
}

// IsDir tells this is a directory
func (l *localFileInfo) IsDir() bool {
	return l.fileInfo.IsDir()
}

// GetName returns file name
func (l *localFileInfo) GetName() string {
	return l.fileInfo.Name()
}

// GetPath returns file path
func (l *localFileInfo) GetPath() string {
	return l.filePath
}

// GetSize returns file size
func (l *localFileInfo) GetSize() int64 {
	return l.fileInfo.Size()
}

// ReadDir returns all files in this directory
func (l *localFileInfo) ReadDir() ([]FileInfo, error) {
	f, err := os.Open(l.filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fis, err := f.Readdir(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	infos := make([]FileInfo, len(fis))
	for i := range fis {
		fi := fis[i]
		info, err := makeFileInfo(filepath.Join(l.GetPath(), fi.Name()))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		infos[i] = info
	}

	return infos, nil
}

// GetModePerm returns file permissions
func (l *localFileInfo) GetModePerm() os.FileMode {
	return l.fileInfo.Mode() & os.ModePerm
}

// GetModTime returns file modification time
func (l *localFileInfo) GetModTime() time.Time {
	return l.fileInfo.ModTime()
}

// GetAccessTime returns file last access time
func (l *localFileInfo) GetAccessTime() time.Time {
	return l.accessTime
}
