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
		return nil, trace.Wrap(err)
	}

	stat, err := os.Stat(basePath)
	if err != nil {
		return nil, trace.Wrap(err)
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
// If the joined path points out of the basePath, an error is returned.
func (s *DirectoryAccess) getSafePath(relativePath string) (string, error) {
	full := filepath.Join(s.basePath, relativePath)
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		// EvalSymlinks returns an error if the target file does not exist.
		// In that case, attempt to resolve the symlinks of the parent directory instead.
		if errors.Is(err, fs.ErrNotExist) {
			parent := filepath.Dir(full)
			resolvedParent, perr := filepath.EvalSymlinks(parent)
			if perr != nil {
				return "", trace.Wrap(perr)
			}

			// Reconstruct the full path by joining the resolved parent with the original file name.
			resolved = filepath.Join(resolvedParent, filepath.Base(full))
		} else {
			return "", trace.Wrap(err)
		}
	}
	if !isSubPath(s.basePath, resolved) {
		return "", trace.BadParameter("path escapes from parent")
	}
	return resolved, nil
}

func isSubPath(parent, child string) bool {
	return child == parent || strings.HasPrefix(child, parent+string(filepath.Separator))
}

// Stat retrieves metadata about a file or directory at the given path.
func (s *DirectoryAccess) Stat(relativePath string) (*FileOrDirInfo, error) {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info, err := s.readFileOrDirInfo(relativePath, stat)
	return info, trace.Wrap(err)
}

// ReadDir lists files and directories within the given directory path, skips symlinks.
func (s *DirectoryAccess) ReadDir(relativePath string) ([]*FileOrDirInfo, error) {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var results []*FileOrDirInfo
	for _, entry := range entries {
		fileInfo, err := entry.Info()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Skip symlinks, we can't present them properly in the remote machine.
		if fileInfo.Mode().Type()&os.ModeSymlink != 0 {
			continue
		}

		entryRelativePath := filepath.Join(relativePath, fileInfo.Name())
		fileOrDir, err := s.readFileOrDirInfo(entryRelativePath, fileInfo)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		results = append(results, fileOrDir)
	}

	return results, nil
}

// Read reads a slice of a file.
func (s *DirectoryAccess) Read(relativePath string, offset int64, length uint32) ([]byte, error) {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opened, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer opened.Close()

	buf := make([]byte, length)
	_, err = opened.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	n, err := opened.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, trace.Wrap(err)
	}
	return buf[:n], nil
}

// Write writes data to a file at a given offset.
func (s *DirectoryAccess) Write(relativePath string, offset int64, data []byte) (int, error) {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return 0, err
	}

	n, err := file.Write(data)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return n, nil
}

// Truncate truncates a file to the specified size.
func (s *DirectoryAccess) Truncate(relativePath string, size int64) error {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.Truncate(path, size))
}

// Create creates a new file or directory at the given path.
func (s *DirectoryAccess) Create(relativePath string, fileType FileType) error {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return trace.Wrap(err)
	}

	switch fileType {
	case FileTypeFile:
		file, err := os.Create(path)
		if err != nil {
			if os.IsExist(err) {
				return nil // Ignore if file already exists
			}
			return err
		}
		return file.Close()
	case FileTypeDir:
		err := os.Mkdir(path, 0700)
		if os.IsExist(err) {
			return nil // Ignore if directory already exists
		}
		return err
	default:
		return errors.New("unknown file type")
	}
}

// Delete removes a file or directory at the given path.
func (s *DirectoryAccess) Delete(relativePath string) error {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.RemoveAll(path)
	return trace.Wrap(err)
}

func (s *DirectoryAccess) readFileOrDirInfo(relativePath string, f os.FileInfo) (*FileOrDirInfo, error) {
	path, err := s.getSafePath(relativePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info := &FileOrDirInfo{
		Size:         f.Size(),
		LastModified: f.ModTime().Unix(),
		Path:         relativePath,
		IsEmpty:      false,
	}

	if f.IsDir() {
		r, err := os.Open(path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer r.Close()
		info.FileType = FileTypeDir
		// Read up to one entry.
		if _, err := r.Readdirnames(1); err != nil {
			if errors.Is(err, io.EOF) {
				info.IsEmpty = true
			} else {
				return nil, trace.Wrap(err)
			}
		}
		info.Size = StandardDirSize
	} else {
		info.FileType = FileTypeFile
	}

	return info, nil
}
