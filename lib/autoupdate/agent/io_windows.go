/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package agent

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

// SetRequiredUmask does nothing on Windows, which does not support umask.
func SetRequiredUmask(ctx context.Context, log *slog.Logger) {
	return
}

func writeAtomicWithinDir(filename string, perm os.FileMode, fn func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = fn(tmp); err != nil {
		return trace.ConvertSystemError(err)
	}
	// Flush the contents before the move, so that a crash can't leave filename
	// pointing at a file whose data never reached the disk.
	if err = tmp.Sync(); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err = tmp.Close(); err != nil {
		return trace.ConvertSystemError(err)
	}
	// There's no way to do atomic rename in Windows, we do best effort here.
	if err = os.Rename(tmp.Name(), filename); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func writeFileAtomicWithinDir(filename string, data []byte, perm os.FileMode) error {
	return writeAtomicWithinDir(filename, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func atomicSymlink(oldpath, newpath string) error {
	return trace.NotImplemented("atomic symlink not supported on Windows")
}

// setJunction creates or replaces a directory junction (NTFS mount point) at link that points to target.
// Unlike symlinks, junctions do not require special privileges to create.
// If link does not exist, it is created. If link is already a junction, its target is replaced in-place
// with a single reparse point update, so that the junction is never missing.
// If link is a directory that is not a junction, setJunction errors unless the directory is empty.
// target must be an absolute path.
func setJunction(link, target string) (err error) {
	if !filepath.IsAbs(target) {
		return trace.BadParameter("junction target %s must be an absolute path", target)
	}
	data, err := mountPointReparseData(target)
	if err != nil {
		return trace.Wrap(err)
	}
	created := false
	if err := os.Mkdir(link, systemDirMode); err == nil {
		created = true
	} else if !errors.Is(err, os.ErrExist) {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		if err != nil && created {
			_ = os.Remove(link)
		}
	}()
	namePtr, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return trace.Wrap(err, "converting path to UTF16")
	}
	h, err := windows.CreateFile(namePtr,
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0)
	if err != nil {
		return trace.Wrap(err, "opening %s", link)
	}
	defer windows.CloseHandle(h)
	var ret uint32
	err = windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &data[0], uint32(len(data)), nil, 0, &ret, nil)
	if err != nil {
		return trace.Wrap(err, "setting junction %s -> %s", link, target)
	}
	return nil
}

// mountPointReparseData encodes a REPARSE_DATA_BUFFER describing a mount point (junction) to target.
//
//	ULONG  ReparseTag
//	USHORT ReparseDataLength
//	USHORT Reserved
//	USHORT SubstituteNameOffset
//	USHORT SubstituteNameLength
//	USHORT PrintNameOffset
//	USHORT PrintNameLength
//	WCHAR  PathBuffer[]
func mountPointReparseData(target string) ([]byte, error) {
	substitute, err := windows.UTF16FromString(`\??\` + target) // NUL-terminated
	if err != nil {
		return nil, trace.Wrap(err, "converting path to UTF16")
	}
	print, err := windows.UTF16FromString(target) // NUL-terminated
	if err != nil {
		return nil, trace.Wrap(err, "converting path to UTF16")
	}
	const headerLen = 8           // ReparseTag + ReparseDataLength + Reserved
	const mountPointHeaderLen = 8 // four USHORT offsets and lengths
	substituteLen := (len(substitute) - 1) * 2
	printLen := (len(print) - 1) * 2
	pathBufferLen := len(substitute)*2 + len(print)*2
	dataLen := mountPointHeaderLen + pathBufferLen
	if headerLen+dataLen > windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE {
		return nil, trace.BadParameter("junction target %s is too long", target)
	}
	le := binary.LittleEndian
	buf := make([]byte, 0, headerLen+dataLen)
	buf = le.AppendUint32(buf, windows.IO_REPARSE_TAG_MOUNT_POINT)
	buf = le.AppendUint16(buf, uint16(dataLen))
	buf = le.AppendUint16(buf, 0) // Reserved
	buf = le.AppendUint16(buf, 0) // SubstituteNameOffset
	buf = le.AppendUint16(buf, uint16(substituteLen))
	buf = le.AppendUint16(buf, uint16(substituteLen+2)) // PrintNameOffset, after NUL
	buf = le.AppendUint16(buf, uint16(printLen))
	for _, c := range substitute {
		buf = le.AppendUint16(buf, c)
	}
	for _, c := range print {
		buf = le.AppendUint16(buf, c)
	}
	return buf, nil
}

// readJunction returns the target of the junction at link.
// readJunction returns os.ErrNotExist if nothing exists at link, and
// ErrFilePresent if a file or directory that is not a junction exists at link.
func readJunction(link string) (string, error) {
	target, err := os.Readlink(link)
	if errors.Is(err, os.ErrNotExist) {
		return "", trace.Wrap(os.ErrNotExist)
	}
	if errors.Is(err, windows.ERROR_NOT_A_REPARSE_POINT) ||
		errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return "", trace.Wrap(ErrFilePresent, "%s is not a junction", link)
	}
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return target, nil
}
