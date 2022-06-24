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

	var sftpEvents []*apievents.SFTP
	sftpSrv, err := sftp.NewServer(ch, sftp.WithRequestCallback(func(reqPacket sftp.RequestPacket, path string, opErr error) {
		event := apievents.SFTP{
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
			event.Action = events.SFTPOpen
			event.Path = p.Path
			event.Flags = p.Pflags
		case *sftp.ClosePacket:
			if opErr == nil {
				event.Code = events.SFTPCloseCode
			} else {
				event.Code = events.SFTPCloseFailureCode
			}
			event.Action = events.SFTPClose
			event.Path = path
		case *sftp.ReadPacket:
			if opErr == nil {
				event.Code = events.SFTPReadCode
			} else {
				event.Code = events.SFTPReadFailureCode
			}
			event.Action = events.SFTPRead
			event.Path = path
		case *sftp.WritePacket:
			if opErr == nil {
				event.Code = events.SFTPWriteCode
			} else {
				event.Code = events.SFTPWriteFailureCode
			}
			event.Action = events.SFTPWrite
			event.Path = path
		case *sftp.LstatPacket:
			if opErr == nil {
				event.Code = events.SFTPLstatCode
			} else {
				event.Code = events.SFTPLstatFailureCode
			}
			event.Action = events.SFTPLstat
			event.Path = p.Path
		case *sftp.FstatPacket:
			if opErr == nil {
				event.Code = events.SFTPFstatCode
			} else {
				event.Code = events.SFTPFstatCode
			}
			event.Action = events.SFTPFstat
			event.Path = path
		case *sftp.SetstatPacket:
			if opErr == nil {
				event.Code = events.SFTPSetstatCode
			} else {
				event.Code = events.SFTPSetstatFailureCode
			}
			event.Action = events.SFTPSetstat
			event.Path = p.Path
			event.Attributes = unmarshalSFTPAttrs(p.Flags, p.Attrs.([]byte))
		case *sftp.FsetstatPacket:
			if opErr == nil {
				event.Code = events.SFTPFsetstatCode
			} else {
				event.Code = events.SFTPFsetstatFailureCode
			}
			event.Action = events.SFTPFsetstat
			event.Path = path
			event.Attributes = unmarshalSFTPAttrs(p.Flags, p.Attrs.([]byte))
		case *sftp.OpendirPacket:
			if opErr == nil {
				event.Code = events.SFTPOpendirCode
			} else {
				event.Code = events.SFTPOpendirFailureCode
			}
			event.Action = events.SFTPOpendir
			event.Path = p.Path
		case *sftp.ReaddirPacket:
			if opErr == nil {
				event.Code = events.SFTPReaddirCode
			} else {
				event.Code = events.SFTPReaddirFailureCode
			}
			event.Action = events.SFTPReaddir
			event.Path = path
		case *sftp.RemovePacket:
			if opErr == nil {
				event.Code = events.SFTPRemoveCode
			} else {
				event.Code = events.SFTPRemoveFailureCode
			}
			event.Action = events.SFTPRemove
			event.Path = p.Filename
		case *sftp.MkdirPacket:
			if opErr == nil {
				event.Code = events.SFTPMkdirCode
			} else {
				event.Code = events.SFTPMkdirFailureCode
			}
			event.Action = events.SFTPMkdir
			event.Path = p.Path
			event.Flags = p.Flags
		case *sftp.RmdirPacket:
			if opErr == nil {
				event.Code = events.SFTPRmdirCode
			} else {
				event.Code = events.SFTPRmdirFailureCode
			}
			event.Action = events.SFTPRmdir
			event.Path = p.Path
		case *sftp.RealpathPacket:
			if opErr == nil {
				event.Code = events.SFTPRealpathCode
			} else {
				event.Code = events.SFTPRealpathFailureCode
			}
			event.Action = events.SFTPRealpath
			event.Path = p.Path
		case *sftp.StatPacket:
			if opErr == nil {
				event.Code = events.SFTPStatCode
			} else {
				event.Code = events.SFTPStatCode
			}
			event.Action = events.SFTPStat
			event.Path = p.Path
		case *sftp.RenamePacket:
			if opErr == nil {
				event.Code = events.SFTPRenameCode
			} else {
				event.Code = events.SFTPRenameFailureCode
			}
			event.Action = events.SFTPRename
			event.Path = p.Oldpath
			event.TargetPath = p.Newpath
		case *sftp.ReadlinkPacket:
			if opErr == nil {
				event.Code = events.SFTPReadlinkCode
			} else {
				event.Code = events.SFTPReadlinkFailureCode
			}
			event.Action = events.SFTPReadlink
			event.Path = p.Path
		case *sftp.SymlinkPacket:
			if opErr == nil {
				event.Code = events.SFTPSymlinkCode
			} else {
				event.Code = events.SFTPSymlinkFailureCode
			}
			event.Action = events.SFTPSymlink
			event.Path = p.Targetpath
			event.TargetPath = p.Linkpath
		default:
			return
		}

		sftpEvents = append(sftpEvents, &event)
	}))
	if err != nil {
		return trace.Wrap(err)
	}

	serveErr := sftpSrv.Serve()
	if errors.Is(serveErr, io.EOF) {
		serveErr = nil
	} else {
		serveErr = trace.Wrap(serveErr)
	}

	eventBytes, err := json.Marshal(sftpEvents)
	if err != nil {
		log.WithError(err).Warn("Failed to marshal SFTP events.")
	} else {
		_, err = io.Copy(auditFile, bytes.NewReader(eventBytes))
		if err != nil {
			log.WithError(err).Warn("Failed to send SFTP events to parent.")
		}
	}

	return trace.NewAggregate(serveErr, sftpSrv.Close())
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
