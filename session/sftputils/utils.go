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
	"errors"
	"io"
	"io/fs"
	"os"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
)

// fileWrapper is a wrapper for *os.File that implements the WriteTo() method
// required for concurrent data transfer.
type fileWrapper struct {
	*os.File
}

func (wt *fileWrapper) WriteTo(w io.Writer) (n int64, err error) {
	return io.Copy(w, wt.File)
}

// TrackedFile is a [File] that counts the bytes read from/written to it.
type TrackedFile struct {
	File
	// BytesRead is the number of bytes read.
	bytesRead atomic.Uint64
	// BytesWritten is the number of bytes written.
	bytesWritten atomic.Uint64
}

func (t *TrackedFile) ReadAt(b []byte, off int64) (int, error) {
	n, err := t.File.ReadAt(b, off)
	t.bytesRead.Add(uint64(n))
	return n, err
}

func (t *TrackedFile) WriteAt(b []byte, off int64) (int, error) {
	n, err := t.File.WriteAt(b, off)
	t.bytesWritten.Add(uint64(n))
	return n, err
}

func (t *TrackedFile) BytesRead() uint64 {
	return t.bytesRead.Load()
}

func (t *TrackedFile) BytesWritten() uint64 {
	return t.bytesWritten.Load()
}

// ParseFlags parses Open flags from an SFTP request to an int as used by
// [os.OpenFile].
func ParseFlags(req *sftp.Request) int {
	pflags := req.Pflags()
	var flags int
	if pflags.Read && pflags.Write {
		flags = os.O_RDWR
	} else if pflags.Read {
		flags = os.O_RDONLY
	} else if pflags.Write {
		flags = os.O_WRONLY
	}

	if pflags.Append {
		flags |= os.O_APPEND
	}
	if pflags.Creat {
		flags |= os.O_CREATE
	}
	if pflags.Excl {
		flags |= os.O_EXCL
	}
	if pflags.Trunc {
		flags |= os.O_TRUNC
	}

	return flags
}

type Event struct {
	SFTP    *SFTPEvent        `json:",omitempty"`
	Summary *SFTPSummaryEvent `json:",omitempty"`
}

type SFTPEvent struct {
	Time    int64
	Method  string
	Error   string `json:",omitempty"`
	Path    string
	Target  string `json:",omitempty"`
	Flags   uint32
	WorkDir string
	Attrs   *SFTPAttributes `json:",omitempty"`
}

type SFTPAttributes struct {
	Atime *uint32 `json:",omitempty"`
	Mtime *uint32 `json:",omitempty"`
	Perms *uint32 `json:",omitempty"`
	Size  *uint64 `json:",omitempty"`
	UID   *uint32 `json:",omitempty"`
	GID   *uint32 `json:",omitempty"`
}

type SFTPSummaryEvent struct {
	Time  int64
	Stats []SummaryFileTransferStat `json:",omitempty"`
}

type SummaryFileTransferStat struct {
	Path    string
	Read    uint64
	Written uint64
}

// ParseSFTPEvent parses an SFTP request and associated error into an SFTP audit
// event. Changes to this function should be reflected in
// [sshutils/sftp.SFTPEventToProto].
func ParseSFTPEvent(req *sftp.Request, workingDirectory string, reqErr error) (*SFTPEvent, error) {
	event := &SFTPEvent{
		Time: time.Now().UnixNano(),
	}

	switch req.Method {
	case MethodOpen, MethodGet, MethodPut:
	case MethodSetStat:
	case MethodList:
	case MethodRemove:
	case MethodMkdir:
	case MethodRmdir:
	case MethodRename:
	case MethodSymlink:
	case MethodLink:
	default:
		return nil, trace.BadParameter("unknown SFTP request %+q", req.Method)
	}

	event.Path = req.Filepath
	event.Target = req.Target
	event.Flags = req.Flags
	event.WorkDir = workingDirectory

	if req.Method == MethodSetStat {
		attrFlags := req.AttrFlags()
		attrs := *req.Attributes()
		event.Attrs = new(SFTPAttributes)

		if attrFlags.Acmodtime {
			event.Attrs.Atime = &attrs.Atime
			event.Attrs.Mtime = &attrs.Mtime
		}
		if attrFlags.Permissions {
			perms := uint32(attrs.FileMode().Perm())
			event.Attrs.Perms = &perms
		}
		if attrFlags.Size {
			event.Attrs.Size = &attrs.Size
		}
		if attrFlags.UidGid {
			event.Attrs.UID = &attrs.UID
			event.Attrs.GID = &attrs.GID
		}
	}

	if reqErr != nil {
		// If possible, strip the filename from the error message. The
		// path will be included in audit events already, no need to
		// make the error message longer than it needs to be.
		var pathErr *fs.PathError
		var linkErr *os.LinkError
		if errors.As(reqErr, &pathErr) {
			event.Error = pathErr.Err.Error()
		} else if errors.As(reqErr, &linkErr) {
			event.Error = linkErr.Err.Error()
		} else {
			event.Error = reqErr.Error()
		}
		if event.Error == "" {
			event.Error = "SFTP request failed with no error message"
		}
	}

	return event, nil
}
