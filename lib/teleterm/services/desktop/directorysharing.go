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
	"math"
	"os"
	"path"
	"path/filepath"

	"github.com/gravitational/trace"
)

// DirectoryAccess enables file system operations for a given directory.
// Should be kept in sync with web/packages/shared/libs/tdp/sharedDirectoryAccess.ts
// where FS events are handled for Web UI.
type DirectoryAccess struct {
	root *os.Root
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
	root, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &DirectoryAccess{root: root}, nil
}

// Close releases the underlying directory handle.
func (d *DirectoryAccess) Close() error {
	return trace.ConvertSystemError(d.root.Close())
}

// rootRel converts the externally-supplied relative path into a form os.Root accepts.
// The RDP-to-TDP conversion in lib/srv/desktop/rdp/rdpclient/src/rdpdr/path.rs maps "\" to "".
// os.Root rejects empty paths, so map "" to "." here (path.Clean does this for us).
func rootRel(relativePath string) string {
	return path.Clean(relativePath)
}

// Stat retrieves metadata about a file or directory at the given path.
func (d *DirectoryAccess) Stat(relativePath string) (*FileOrDirInfo, error) {
	stat, err := d.root.Stat(rootRel(relativePath))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	info, err := d.readFileOrDirInfo(relativePath, stat)
	return info, trace.Wrap(err)
}

// ReadDir lists files and directories within the given directory path, skips symlinks.
func (d *DirectoryAccess) ReadDir(relativePath string) ([]*FileOrDirInfo, error) {
	dir, err := d.root.Open(rootRel(relativePath))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	var results []*FileOrDirInfo
	for _, entry := range entries {
		// Skip symlinks, we can't present them properly in the remote machine.
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return nil, trace.ConvertSystemError(err)
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
	file, err := d.root.Open(rootRel(relativePath))
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
	file, err := d.root.OpenFile(rootRel(relativePath), os.O_RDWR|os.O_CREATE, 0644)
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
func (d *DirectoryAccess) Truncate(relativePath string, size uint64) error {
	if size > math.MaxInt64 {
		size = math.MaxInt64
	}

	file, err := d.root.OpenFile(rootRel(relativePath), os.O_WRONLY, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()

	return trace.ConvertSystemError(file.Truncate(int64(size)))
}

// Create creates a new file or directory at the given path.
func (d *DirectoryAccess) Create(relativePath string, fileType FileType) error {
	path := rootRel(relativePath)
	switch fileType {
	case FileTypeFile:
		file, err := d.root.Create(path)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				return nil // Ignore if file already exists
			}
			return trace.ConvertSystemError(err)
		}
		return trace.ConvertSystemError(file.Close())
	case FileTypeDir:
		err := d.root.Mkdir(path, 0700)
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
	err := d.root.RemoveAll(rootRel(relativePath))
	return trace.ConvertSystemError(err)
}

func (d *DirectoryAccess) readFileOrDirInfo(relativePath string, f os.FileInfo) (info *FileOrDirInfo, err error) {
	info = &FileOrDirInfo{
		Size:         f.Size(),
		LastModified: f.ModTime().UnixMilli(),
		Path:         relativePath,
		IsEmpty:      false,
	}

	if !f.IsDir() {
		info.FileType = FileTypeFile
		return info, nil
	}

	opened, err := d.root.Open(rootRel(relativePath))
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
