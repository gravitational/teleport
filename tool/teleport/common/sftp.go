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
	"errors"
	"io"
	"os"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
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
	chr, err := openFD(3, "chr")
	if err != nil {
		return err
	}
	defer chr.Close()
	chw, err := openFD(4, "chw")
	if err != nil {
		return err
	}
	defer chw.Close()
	ch := compositeCh{chr, chw}
	auditFile, err := openFD(5, "audit")
	if err != nil {
		return err
	}
	defer auditFile.Close()

	// Ensure the parent process will receive log messages from us
	utils.InitLogger(utils.LoggingForDaemon, log.InfoLevel)

	sftpEvents := make(chan *apievents.SFTP, 1)
	sftpSrv, err := sftp.NewServer(ch, sftp.WithRequestCallback(func(reqPacket sftp.RequestPacket) {
		event, ok := handleSFTPEvent(reqPacket)
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
		var m jsonpb.Marshaler
		for event := range sftpEvents {
			eventStr, err := m.MarshalToString(event)
			if err != nil {
				log.WithError(err).Warn("Failed to marshal SFTP event.")
			} else {
				// Append a NULL byte so the parent process will know where
				// this event ends
				eventBytes := []byte(eventStr)
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

	// Wait until event marshaling goroutine is finished
	close(sftpEvents)
	<-done

	return trace.NewAggregate(serveErr, sftpSrv.Close())
}

func openFD(fd uintptr, name string) (*os.File, error) {
	ret, err := unix.FcntlInt(fd, unix.F_GETFD, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ret != 0 {
		return nil, trace.BadParameter("'%s sftp' should not be run directly. It will be executed by Teleport when SFTP connections are received.", os.Args[0])
	}
	file := os.NewFile(fd, name)
	if file == nil {
		return nil, trace.NotFound("inherited file %s not found", name)
	}

	return file, nil
}

func handleSFTPEvent(reqPacket sftp.RequestPacket) (*apievents.SFTP, bool) {
	event := &apievents.SFTP{
		Metadata: apievents.Metadata{
			Type: events.SFTPEvent,
			Time: time.Now(),
		},
	}

	switch reqPacket.Type {
	case sftp.Open:
		if reqPacket.Err == nil {
			event.Code = events.SFTPOpenCode
		} else {
			event.Code = events.SFTPOpenFailureCode
		}
		event.Action = apievents.SFTPAction_OPEN
	case sftp.Close:
		if reqPacket.Err == nil {
			event.Code = events.SFTPCloseCode
		} else {
			event.Code = events.SFTPCloseFailureCode
		}
		event.Action = apievents.SFTPAction_CLOSE
	case sftp.Read:
		if reqPacket.Err == nil {
			event.Code = events.SFTPReadCode
		} else {
			event.Code = events.SFTPReadFailureCode
		}
		event.Action = apievents.SFTPAction_READ
	case sftp.Write:
		if reqPacket.Err == nil {
			event.Code = events.SFTPWriteCode
		} else {
			event.Code = events.SFTPWriteFailureCode
		}
		event.Action = apievents.SFTPAction_WRITE
	case sftp.Lstat:
		if reqPacket.Err == nil {
			event.Code = events.SFTPLstatCode
		} else {
			event.Code = events.SFTPLstatFailureCode
		}
		event.Action = apievents.SFTPAction_LSTAT
	case sftp.Fstat:
		if reqPacket.Err == nil {
			event.Code = events.SFTPFstatCode
		} else {
			event.Code = events.SFTPFstatFailureCode
		}
		event.Action = apievents.SFTPAction_FSTAT
	case sftp.Setstat:
		if reqPacket.Err == nil {
			event.Code = events.SFTPSetstatCode
		} else {
			event.Code = events.SFTPSetstatFailureCode
		}
		event.Action = apievents.SFTPAction_SETSTAT
	case sftp.Fsetstat:
		if reqPacket.Err == nil {
			event.Code = events.SFTPFsetstatCode
		} else {
			event.Code = events.SFTPFsetstatFailureCode
		}
		event.Action = apievents.SFTPAction_FSETSTAT
	case sftp.Opendir:
		if reqPacket.Err == nil {
			event.Code = events.SFTPOpendirCode
		} else {
			event.Code = events.SFTPOpendirFailureCode
		}
		event.Action = apievents.SFTPAction_OPENDIR
	case sftp.Readdir:
		if reqPacket.Err == nil {
			event.Code = events.SFTPReaddirCode
		} else {
			event.Code = events.SFTPReaddirFailureCode
		}
		event.Action = apievents.SFTPAction_READDIR
	case sftp.Remove:
		if reqPacket.Err == nil {
			event.Code = events.SFTPRemoveCode
		} else {
			event.Code = events.SFTPRemoveFailureCode
		}
		event.Action = apievents.SFTPAction_REMOVE
	case sftp.Mkdir:
		if reqPacket.Err == nil {
			event.Code = events.SFTPMkdirCode
		} else {
			event.Code = events.SFTPMkdirFailureCode
		}
		event.Action = apievents.SFTPAction_MKDIR
	case sftp.Rmdir:
		if reqPacket.Err == nil {
			event.Code = events.SFTPRmdirCode
		} else {
			event.Code = events.SFTPRmdirFailureCode
		}
		event.Action = apievents.SFTPAction_RMDIR
	case sftp.Realpath:
		if reqPacket.Err == nil {
			event.Code = events.SFTPRealpathCode
		} else {
			event.Code = events.SFTPRealpathFailureCode
		}
		event.Action = apievents.SFTPAction_REALPATH
	case sftp.Stat:
		if reqPacket.Err == nil {
			event.Code = events.SFTPStatCode
		} else {
			event.Code = events.SFTPStatFailureCode
		}
		event.Action = apievents.SFTPAction_STAT
	case sftp.Rename:
		if reqPacket.Err == nil {
			event.Code = events.SFTPRenameCode
		} else {
			event.Code = events.SFTPRenameFailureCode
		}
		event.Action = apievents.SFTPAction_RENAME
	case sftp.Readlink:
		if reqPacket.Err == nil {
			event.Code = events.SFTPReadlinkCode
		} else {
			event.Code = events.SFTPReadlinkFailureCode
		}
		event.Action = apievents.SFTPAction_READLINK
	case sftp.Symlink:
		if reqPacket.Err == nil {
			event.Code = events.SFTPSymlinkCode
		} else {
			event.Code = events.SFTPSymlinkFailureCode
		}
		event.Action = apievents.SFTPAction_SYMLINK
	default:
		return nil, false
	}

	wd, err := os.Getwd()
	if err != nil {
		log.WithError(err).Warn("Failed to get working dir.")
	}

	event.WorkingDirectory = wd
	event.Path = reqPacket.Path
	event.TargetPath = reqPacket.TargetPath
	event.Flags = reqPacket.Flags
	if reqPacket.Attributes != nil {
		event.Attributes = &apievents.SFTPAttributes{
			AccessTime:       reqPacket.Attributes.AccessTime,
			ModificationTime: reqPacket.Attributes.ModificationTime,
		}
		if reqPacket.Attributes.Size != nil {
			event.Attributes.FileSize = reqPacket.Attributes.Size
		}
		if reqPacket.Attributes.UID != nil {
			event.Attributes.UID = reqPacket.Attributes.UID
		}
		if reqPacket.Attributes.GID != nil {
			event.Attributes.GID = reqPacket.Attributes.GID
		}
		if reqPacket.Attributes.Permissions != nil {
			event.Attributes.Permissions = (*uint32)(reqPacket.Attributes.Permissions)
		}
	}

	return event, true
}
