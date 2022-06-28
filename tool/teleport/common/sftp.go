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

package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
)

type compositeCh struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (c compositeCh) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

func (c compositeCh) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

func (c compositeCh) Close() error {
	return trace.NewAggregate(c.r.Close(), c.w.Close())
}

func onSFTP() error {
	// Ensure the parent process will receive log messages from us
	utils.InitLogger(utils.LoggingForDaemon, log.InfoLevel)

	chr := os.NewFile(3, "chr")
	if chr == nil {
		return trace.NotFound("channel read file not found")
	}
	defer chr.Close()
	chw := os.NewFile(4, "chw")
	if chw == nil {
		return trace.NotFound("channel write file not found")
	}
	defer chw.Close()
	ch := compositeCh{chr, chw}
	auditFile := os.NewFile(5, "audit")
	if auditFile == nil {
		return trace.NotFound("audit write file not found")
	}
	defer auditFile.Close()

	sftpEvents := make(chan *apievents.SFTP, 8)
	sftpSrv, err := sftp.NewServer(ch, sftp.WithRequestCallback(func(reqPacket sftp.RequestPacket, path string, opErr error) {
		event, ok := handleSFTPEvent(reqPacket, path, opErr)
		if !ok {
			// We don't care about this type of SFTP request, move on
			return
		}

		sftpEvents <- event
	}))
	if err != nil {
		return trace.Wrap(err)
	}

	// Start a goroutine to marshal and send audit events to the parent
	// process to avoid blocking the SFTP connection on event handling
	done := make(chan struct{})
	go func() {
		for event := range sftpEvents {
			eventBytes, err := json.Marshal(event)
			if err != nil {
				log.WithError(err).Warn("Failed to marshal SFTP event.")
			} else {
				// Append a NULL byte so the parent process will know where
				// this event ends
				eventBytes = append(eventBytes, 0x0)
				_, err = io.Copy(auditFile, bytes.NewReader(eventBytes))
				if err != nil {
					log.WithError(err).Warn("Failed to send SFTP event to parent.")
				}
			}
		}

		close(done)
	}()

	serveErr := sftpSrv.Serve()
	if errors.Is(serveErr, io.EOF) {
		serveErr = nil
	} else {
		serveErr = trace.Wrap(serveErr)
	}

	// Wait until event marshalling goroutine is finished
	close(sftpEvents)
	<-done

	return trace.NewAggregate(serveErr, sftpSrv.Close())
}

func handleSFTPEvent(reqPacket sftp.RequestPacket, path string, opErr error) (*apievents.SFTP, bool) {
	event := &apievents.SFTP{
		Metadata: apievents.Metadata{
			Type: events.SFTPEvent,
			Time: time.Now(),
		},
	}

	switch p := reqPacket.(type) {
	case *sftp.OpenPacket:
		if opErr == nil {
			event.Code = events.SFTPOpenCode
		} else {
			event.Code = events.SFTPOpenFailureCode
		}
		event.Action = apievents.SFTPAction_OPEN
		event.Path = p.Path
		event.Flags = p.Pflags
	case *sftp.ClosePacket:
		if opErr == nil {
			event.Code = events.SFTPCloseCode
		} else {
			event.Code = events.SFTPCloseFailureCode
		}
		event.Action = apievents.SFTPAction_CLOSE
		event.Path = path
	case *sftp.ReadPacket:
		if opErr == nil {
			event.Code = events.SFTPReadCode
		} else {
			event.Code = events.SFTPReadFailureCode
		}
		event.Action = apievents.SFTPAction_READ
		event.Path = path
	case *sftp.WritePacket:
		if opErr == nil {
			event.Code = events.SFTPWriteCode
		} else {
			event.Code = events.SFTPWriteFailureCode
		}
		event.Action = apievents.SFTPAction_WRITE
		event.Path = path
	case *sftp.LstatPacket:
		if opErr == nil {
			event.Code = events.SFTPLstatCode
		} else {
			event.Code = events.SFTPLstatFailureCode
		}
		event.Action = apievents.SFTPAction_LSTAT
		event.Path = p.Path
	case *sftp.FstatPacket:
		if opErr == nil {
			event.Code = events.SFTPFstatCode
		} else {
			event.Code = events.SFTPFstatFailureCode
		}
		event.Action = apievents.SFTPAction_FSTAT
		event.Path = path
	case *sftp.SetstatPacket:
		if opErr == nil {
			event.Code = events.SFTPSetstatCode
		} else {
			event.Code = events.SFTPSetstatFailureCode
		}
		event.Action = apievents.SFTPAction_SETSTAT
		event.Path = p.Path
		event.Attributes = unmarshalSFTPAttrs(p.Flags, p.Attrs.([]byte))
	case *sftp.FsetstatPacket:
		if opErr == nil {
			event.Code = events.SFTPFsetstatCode
		} else {
			event.Code = events.SFTPFsetstatFailureCode
		}
		event.Action = apievents.SFTPAction_FSETSTAT
		event.Path = path
		event.Attributes = unmarshalSFTPAttrs(p.Flags, p.Attrs.([]byte))
	case *sftp.OpendirPacket:
		if opErr == nil {
			event.Code = events.SFTPOpendirCode
		} else {
			event.Code = events.SFTPOpendirFailureCode
		}
		event.Action = apievents.SFTPAction_OPENDIR
		event.Path = p.Path
	case *sftp.ReaddirPacket:
		if opErr == nil {
			event.Code = events.SFTPReaddirCode
		} else {
			event.Code = events.SFTPReaddirFailureCode
		}
		event.Action = apievents.SFTPAction_READDIR
		event.Path = path
	case *sftp.RemovePacket:
		if opErr == nil {
			event.Code = events.SFTPRemoveCode
		} else {
			event.Code = events.SFTPRemoveFailureCode
		}
		event.Action = apievents.SFTPAction_REMOVE
		event.Path = p.Filename
	case *sftp.MkdirPacket:
		if opErr == nil {
			event.Code = events.SFTPMkdirCode
		} else {
			event.Code = events.SFTPMkdirFailureCode
		}
		event.Action = apievents.SFTPAction_MKDIR
		event.Path = p.Path
		event.Flags = p.Flags
	case *sftp.RmdirPacket:
		if opErr == nil {
			event.Code = events.SFTPRmdirCode
		} else {
			event.Code = events.SFTPRmdirFailureCode
		}
		event.Action = apievents.SFTPAction_RMDIR
		event.Path = p.Path
	case *sftp.RealpathPacket:
		if opErr == nil {
			event.Code = events.SFTPRealpathCode
		} else {
			event.Code = events.SFTPRealpathFailureCode
		}
		event.Action = apievents.SFTPAction_REALPATH
		event.Path = p.Path
	case *sftp.StatPacket:
		if opErr == nil {
			event.Code = events.SFTPStatCode
		} else {
			event.Code = events.SFTPStatFailureCode
		}
		event.Action = apievents.SFTPAction_STAT
		event.Path = p.Path
	case *sftp.RenamePacket:
		if opErr == nil {
			event.Code = events.SFTPRenameCode
		} else {
			event.Code = events.SFTPRenameFailureCode
		}
		event.Action = apievents.SFTPAction_RENAME
		event.Path = p.Oldpath
		event.TargetPath = p.Newpath
	case *sftp.ReadlinkPacket:
		if opErr == nil {
			event.Code = events.SFTPReadlinkCode
		} else {
			event.Code = events.SFTPReadlinkFailureCode
		}
		event.Action = apievents.SFTPAction_READLINK
		event.Path = p.Path
	case *sftp.SymlinkPacket:
		if opErr == nil {
			event.Code = events.SFTPSymlinkCode
		} else {
			event.Code = events.SFTPSymlinkFailureCode
		}
		event.Action = apievents.SFTPAction_SYMLINK
		event.Path = p.Targetpath
		event.TargetPath = p.Linkpath
	default:
		return nil, false
	}

	return event, true
}

const (
	sftpSizeAttr      uint32 = 1
	sftpUIDGIDAttr    uint32 = 2
	sftpPermsAttr     uint32 = 4
	sftpACMODTimeAttr uint32 = 8
)

func unmarshalSFTPAttrs(flags uint32, b []byte) *apievents.SFTPAttributes {
	var attrs apievents.SFTPAttributes
	if flags&sftpSizeAttr != 0 {
		attrs.Size_ = binary.BigEndian.Uint64(b)
		b = b[8:]
	}
	if flags&sftpPermsAttr != 0 {
		attrs.Permissions = binary.BigEndian.Uint32(b)
		b = b[4:]
	}
	if flags&sftpACMODTimeAttr != 0 {
		atime := binary.BigEndian.Uint32(b)
		b = b[4:]
		mtime := binary.BigEndian.Uint32(b)
		b = b[4:]

		atimeT := time.Unix(int64(atime), 0)
		mtimeT := time.Unix(int64(mtime), 0)
		attrs.AccessTime = &atimeT
		attrs.ModificationTime = &mtimeT
	}
	if flags&sftpUIDGIDAttr != 0 {
		attrs.UID = binary.BigEndian.Uint32(b)
		b = b[4:]
		attrs.GID = binary.BigEndian.Uint32(b)
	}

	return &attrs
}
