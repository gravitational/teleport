// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package desktop

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

// DirectoryAccess enables file system operations for a given directory.
// Should be kept in sync with web/packages/shared/libs/tdp/sharedDirectoryAccess.ts
// where FS events are handled for Web UI.
type DirectoryAccess struct {
	// basePath is a shared directory path.
	// Must not be used directly, but only through getSafePath.
	// TODO(gzdunek): This code can be greatly simplified with os.OpenRoot.
	// Switch to it when branch/v17 is updated to Go 1.24.
	basePath string
}

// FileOrDirInfo contains metadata about a file or a directory.
type FileOrDirInfo struct {
	Size         int64
	LastModified int64
	FileType     FileType // "file" or "directory"
	IsEmpty      bool
	Path         string
}

type FileType uint32

const (
	FileTypeFile FileType = iota
	FileTypeDir
)

const StandardDirSize = 4096

// NewDirectoryAccess initializes a DirectoryAccess instance for the given directory.
func NewDirectoryAccess(baseDir string) (*DirectoryAccess, error) {
	basePath, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	stat, err := os.Stat(basePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if !stat.IsDir() {
		return nil, trace.BadParameter("%q is not a directory", baseDir)
	}

	return &DirectoryAccess{
		basePath: basePath,
	}, nil

}

// getSafePath allows building a safe path by joining the base path
// and a relative path.
// Returns an error if the resolved path escapes the basePath, preventing directory traversal attacks.
func (d *DirectoryAccess) getSafePath(relativePath string) (string, error) {
	full := filepath.Join(d.basePath, relativePath)
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		// EvalSymlinks returns an error if the target file does not exist.
		// In that case, attempt to resolve the symlinks of the parent directory instead.
		if errors.Is(err, fs.ErrNotExist) {
			parent := filepath.Dir(full)
			resolvedParent, perr := filepath.EvalSymlinks(parent)
			if perr != nil {
				return "", trace.ConvertSystemError(perr)
			}

			// Reconstruct the full path by joining the resolved parent with the original file name.
			resolved = filepath.Join(resolvedParent, filepath.Base(full))
		} else {
			return "", trace.ConvertSystemError(err)
		}
	}
	if !isSubPath(d.basePath, resolved) {
		return "", trace.BadParameter("path escapes from parent")
	}
	return resolved, nil
}

func isSubPath(parent, child string) bool {
	return child == parent || strings.HasPrefix(child, parent+string(filepath.Separator))
}

// Stat retrieves metadata about a file or directory at the given path.
func (d *DirectoryAccess) Stat(relativePath string) (*FileOrDirInfo, error) {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	info, err := d.readFileOrDirInfo(relativePath, stat)
	return info, trace.Wrap(err)
}

// ReadDir lists files and directories within the given directory path, skips symlinks.
func (d *DirectoryAccess) ReadDir(relativePath string) ([]*FileOrDirInfo, error) {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	var results []*FileOrDirInfo
	for _, entry := range entries {
		fileInfo, err := entry.Info()
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}

		// Skip symlinks, we can't present them properly in the remote machine.
		if fileInfo.Mode().Type()&os.ModeSymlink != 0 {
			continue
		}

		entryRelativePath := filepath.Join(relativePath, fileInfo.Name())
		fileOrDir, err := d.readFileOrDirInfo(entryRelativePath, fileInfo)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		results = append(results, fileOrDir)
	}

	return results, nil
}

// Read reads a slice of a file into buf. Returns the number of read bytes.
func (d *DirectoryAccess) Read(relativePath string, offset int64, buf []byte) (n int, err error) {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}
	defer func() {
		closeErr := file.Close()
		if err == nil {
			// Only update err if no previous error occurred.
			err = trace.ConvertSystemError(closeErr)
		}
	}()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}

	n, err = file.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, trace.ConvertSystemError(err)
	}
	return n, err
}

// Write writes data to a file at a given offset.
func (d *DirectoryAccess) Write(relativePath string, offset int64, data []byte) (n int, err error) {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}

	defer func() {
		closeErr := file.Close()
		if err == nil {
			// Only update err if no previous error occurred.
			err = trace.ConvertSystemError(closeErr)
		}
	}()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}

	n, err = file.Write(data)
	return n, trace.ConvertSystemError(err)
}

// Truncate truncates a file to the specified size.
func (d *DirectoryAccess) Truncate(relativePath string, size int64) error {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.ConvertSystemError(os.Truncate(path, size))
}

// Create creates a new file or directory at the given path.
func (d *DirectoryAccess) Create(relativePath string, fileType FileType) error {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return trace.Wrap(err)
	}

	switch fileType {
	case FileTypeFile:
		file, err := os.Create(path)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				return nil // Ignore if file already exists
			}
			return trace.ConvertSystemError(err)
		}
		return trace.ConvertSystemError(file.Close())
	case FileTypeDir:
		err := os.Mkdir(path, 0700)
		if errors.Is(err, fs.ErrExist) {
			return nil // Ignore if directory already exists
		}
		return trace.ConvertSystemError(err)
	default:
		return trace.BadParameter("unknown file type")
	}
}

// Delete removes a file or directory at the given path.
func (d *DirectoryAccess) Delete(relativePath string) error {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.RemoveAll(path)
	return trace.ConvertSystemError(err)
}

func (d *DirectoryAccess) readFileOrDirInfo(relativePath string, f os.FileInfo) (info *FileOrDirInfo, err error) {
	path, err := d.getSafePath(relativePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info = &FileOrDirInfo{
		Size:         f.Size(),
		LastModified: f.ModTime().Unix(),
		Path:         relativePath,
		IsEmpty:      false,
	}

	if !f.IsDir() {
		info.FileType = FileTypeFile
		return info, nil
	}

	opened, err := os.Open(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer func() {
		closeErr := opened.Close()
		if err == nil {
			// Only update err if no previous error occurred.
			err = trace.ConvertSystemError(closeErr)
		}
	}()

	info.FileType = FileTypeDir
	info.Size = StandardDirSize

	// Determine if the dir is not empty by checking if it contains at least one file.
	_, err = opened.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		err = nil
		info.IsEmpty = true
	}
	return info, trace.ConvertSystemError(err)
}
