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

package sftputils

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
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

// PathExpansionError is an [error] indicating that
// path expansion was rejected.
type PathExpansionError struct {
	path string
}

func (p PathExpansionError) Error() string {
	return fmt.Sprintf("expanding remote ~user paths is not supported, specify an absolute path instead of %q", p.path)
}

// ExpandHomeDir evaluates the home directory ('~') in a path.
func ExpandHomeDir(pathStr string) (string, error) {
	pfxLen, ok := homeDirPrefixLen(pathStr)
	if !ok {
		return pathStr, nil
	}

	if pfxLen == 1 && len(pathStr) > 1 {
		return "", trace.Wrap(PathExpansionError{path: pathStr})
	}

	// if an SFTP path is not absolute, it is assumed to start at the user's
	// home directory so just strip the prefix and let the SFTP server
	// figure out the correct remote path.
	trimmedPath := pathStr[pfxLen:]
	// Returning an empty string is supported by SFTP but won't be as clear in
	// logs or audit events. Since the SFTP server will be rooted at the user's
	// home directory, "." and "" are equivalent in this context.
	if trimmedPath == "" {
		return ".", nil
	}
	return trimmedPath, nil
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
