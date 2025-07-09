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

package sftp

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// fileWrapper is a wrapper for *os.File that implements the WriteTo() method
// required for concurrent data transfer.
type fileWrapper struct {
	*os.File
}

func (wt *fileWrapper) WriteTo(w io.Writer) (n int64, err error) {
	return io.Copy(w, wt.File)
}

// fileStreamReader is a thin wrapper around fs.File with additional streams.
type fileStreamReader struct {
	ctx     context.Context
	streams []io.Reader
	file    fs.File
}

// Stat returns file stats.
func (r *fileStreamReader) Stat() (os.FileInfo, error) {
	return r.file.Stat()
}

// Read reads the data from a file and passes the read data to all readers.
// All errors from stream are returned except io.EOF.
func (r *fileStreamReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := r.file.Read(b)
	// Create a copy as not whole buffer can be filled.
	readBuff := b[:n]

	for _, stream := range r.streams {
		if _, innerError := stream.Read(readBuff); innerError != nil {
			// Ignore EOF
			if !errors.Is(err, io.EOF) {
				return 0, innerError
			}
		}
	}

	return n, err
}

// cancelWriter implements io.Writer interface with context cancellation.
type cancelWriter struct {
	ctx    context.Context
	stream io.Writer
}

func (c *cancelWriter) Write(b []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.stream.Write(b)
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

// ParseSFTPEvent parses an SFTP request and associated error into an SFTP
// audit event.
func ParseSFTPEvent(req *sftp.Request, workingDirectory string, reqErr error) (*apievents.SFTP, error) {
	event := &apievents.SFTP{
		Metadata: apievents.Metadata{
			Type: events.SFTPEvent,
			Time: time.Now(),
		},
	}

	switch req.Method {
	case MethodOpen, MethodGet, MethodPut:
		if reqErr == nil {
			event.Code = events.SFTPOpenCode
		} else {
			event.Code = events.SFTPOpenFailureCode
		}
		event.Action = apievents.SFTPAction_OPEN
	case MethodSetStat:
		if reqErr == nil {
			event.Code = events.SFTPSetstatCode
		} else {
			event.Code = events.SFTPSetstatFailureCode
		}
		event.Action = apievents.SFTPAction_SETSTAT
	case MethodList:
		if reqErr == nil {
			event.Code = events.SFTPReaddirCode
		} else {
			event.Code = events.SFTPReaddirFailureCode
		}
		event.Action = apievents.SFTPAction_READDIR
	case MethodRemove:
		if reqErr == nil {
			event.Code = events.SFTPRemoveCode
		} else {
			event.Code = events.SFTPRemoveFailureCode
		}
		event.Action = apievents.SFTPAction_REMOVE
	case MethodMkdir:
		if reqErr == nil {
			event.Code = events.SFTPMkdirCode
		} else {
			event.Code = events.SFTPMkdirFailureCode
		}
		event.Action = apievents.SFTPAction_MKDIR
	case MethodRmdir:
		if reqErr == nil {
			event.Code = events.SFTPRmdirCode
		} else {
			event.Code = events.SFTPRmdirFailureCode
		}
		event.Action = apievents.SFTPAction_RMDIR
	case MethodRename:
		if reqErr == nil {
			event.Code = events.SFTPRenameCode
		} else {
			event.Code = events.SFTPRenameFailureCode
		}
		event.Action = apievents.SFTPAction_RENAME
	case MethodSymlink:
		if reqErr == nil {
			event.Code = events.SFTPSymlinkCode
		} else {
			event.Code = events.SFTPSymlinkFailureCode
		}
		event.Action = apievents.SFTPAction_SYMLINK
	case MethodLink:
		if reqErr == nil {
			event.Code = events.SFTPLinkCode
		} else {
			event.Code = events.SFTPLinkFailureCode
		}
		event.Action = apievents.SFTPAction_LINK
	default:
		return nil, trace.BadParameter("unknown SFTP request %q", req.Method)
	}

	event.Path = req.Filepath
	event.TargetPath = req.Target
	event.Flags = req.Flags
	event.WorkingDirectory = workingDirectory
	if req.Method == MethodSetStat {
		attrFlags := req.AttrFlags()
		attrs := req.Attributes()
		event.Attributes = new(apievents.SFTPAttributes)

		if attrFlags.Acmodtime {
			atime := time.Unix(int64(attrs.Atime), 0)
			mtime := time.Unix(int64(attrs.Mtime), 0)
			event.Attributes.AccessTime = &atime
			event.Attributes.ModificationTime = &mtime
		}
		if attrFlags.Permissions {
			perms := uint32(attrs.FileMode().Perm())
			event.Attributes.Permissions = &perms
		}
		if attrFlags.Size {
			event.Attributes.FileSize = &attrs.Size
		}
		if attrFlags.UidGid {
			event.Attributes.UID = &attrs.UID
			event.Attributes.GID = &attrs.GID
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
	}

	return event, nil
}
