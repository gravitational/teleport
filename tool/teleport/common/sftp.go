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

package common

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed for backwards compatibility
	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
	sftputils "github.com/gravitational/teleport/lib/sshutils/sftp"
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

type allowedOps struct {
	write bool
	path  string
}

// sftpHandler provides handlers for a SFTP server.
type sftpHandler struct {
	logger  *slog.Logger
	allowed *allowedOps

	// mtx protects files
	mtx   sync.Mutex
	files []*sftputils.TrackedFile

	events chan<- apievents.AuditEvent
}

func newSFTPHandler(logger *slog.Logger, req *srv.FileTransferRequest, events chan<- apievents.AuditEvent) (*sftpHandler, error) {
	var allowed *allowedOps
	if req != nil {
		allowed = &allowedOps{
			write: !req.Download,
		}
		// TODO(capnspacehook): reject relative paths and symlinks
		// make filepaths consistent by ensuring all separators use backslashes
		allowed.path = path.Clean(req.Location)
	}

	return &sftpHandler{
		logger:  logger,
		allowed: allowed,
		events:  events,
	}, nil
}

func newDisallowedErr(req *sftp.Request) error {
	return fmt.Errorf("method %s is not allowed on %s", strings.ToLower(req.Method), req.Filepath)
}

// ensureReqIsAllowed returns an error if the SFTP request isn't
// allowed based on the approved file transfer request for this session.
func (s *sftpHandler) ensureReqIsAllowed(req *sftp.Request) error {
	// no specifically allowed operations, all requests are allowed
	if s.allowed == nil {
		return nil
	}

	if s.allowed.path != path.Clean(req.Filepath) {
		return newDisallowedErr(req)
	}

	switch req.Method {
	case sftputils.MethodLstat, sftputils.MethodStat:
		// these methods are allowed
	case sftputils.MethodGet:
		// only allow reads for downloads
		if s.allowed.write {
			return newDisallowedErr(req)
		}
	case sftputils.MethodPut, sftputils.MethodSetStat:
		// only allow writes and chmods for uploads
		if !s.allowed.write {
			return newDisallowedErr(req)
		}
	default:
		return newDisallowedErr(req)
	}

	return nil
}

// OpenFile handles Open requests for both reading and writing. Required to
// satisfy [sftp.OpenFileWriter].
func (s *sftpHandler) OpenFile(req *sftp.Request) (_ sftp.WriterAtReaderAt, retErr error) {
	defer s.sendSFTPEvent(req, retErr)

	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}

	return s.openFile(req)
}

// Fileread handles 'open' requests when opening a file for reading
// is desired.
func (s *sftpHandler) Fileread(req *sftp.Request) (_ io.ReaderAt, retErr error) {
	defer s.sendSFTPEvent(req, retErr)

	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if !req.Pflags().Read {
		return nil, os.ErrInvalid
	}

	return s.openFile(req)
}

// Filewrite handles 'open' requests when opening a file for writing
// is desired.
func (s *sftpHandler) Filewrite(req *sftp.Request) (_ io.WriterAt, retErr error) {
	defer s.sendSFTPEvent(req, retErr)

	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if !req.Pflags().Write {
		return nil, os.ErrInvalid
	}

	return s.openFile(req)
}

func (s *sftpHandler) openFile(req *sftp.Request) (sftp.WriterAtReaderAt, error) {
	if err := s.ensureReqIsAllowed(req); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(req.Filepath, sftputils.ParseFlags(req), defaults.FilePermissions)
	if err != nil {
		return nil, err
	}
	trackFile := &sftputils.TrackedFile{File: f}
	s.mtx.Lock()
	s.files = append(s.files, trackFile)
	s.mtx.Unlock()

	return trackFile, nil
}

// Filecmd handles file modification requests.
func (s *sftpHandler) Filecmd(req *sftp.Request) (retErr error) {
	defer func() {
		if errors.Is(retErr, sftp.ErrSSHFxOpUnsupported) {
			return
		}
		s.sendSFTPEvent(req, retErr)
	}()

	if req.Filepath == "" {
		return os.ErrInvalid
	}
	if err := s.ensureReqIsAllowed(req); err != nil {
		return err
	}

	return sftputils.HandleFilecmd(req, nil /* local filesystem */)
}

// Filelist handles 'readdir', 'stat' and 'readlink' requests.
func (s *sftpHandler) Filelist(req *sftp.Request) (_ sftp.ListerAt, retErr error) {
	defer func() {
		if req.Method == sftputils.MethodList {
			s.sendSFTPEvent(req, retErr)
		}
	}()

	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if err := s.ensureReqIsAllowed(req); err != nil {
		return nil, err
	}

	return sftputils.HandleFilelist(req, nil /* local filesystem */)
}

// RealPath canonicalizes a path name, including resolving ".." and
// following symlinks. Required to implement [sftp.RealPathFileLister].
func (s *sftpHandler) RealPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func (s *sftpHandler) sendSFTPEvent(req *sftp.Request, reqErr error) {
	wd, err := os.Getwd()
	if err != nil {
		s.logger.WarnContext(req.Context(), "Failed to get working dir", "error", err)
		// Emit event without working directory.
	}
	event, err := sftputils.ParseSFTPEvent(req, wd, reqErr)
	if err != nil {
		s.logger.WarnContext(req.Context(), "Unknown SFTP request", "request", req.Method)
		return
	} else if reqErr != nil {
		s.logger.DebugContext(req.Context(), "failed handling SFTP request", "request", req.Method, "error", reqErr)
	}
	s.events <- event
}

func onSFTP() error {
	chr, err := openFD(3, "chr")
	if err != nil {
		return trace.Wrap(err)
	}
	defer chr.Close()
	chw, err := openFD(4, "chw")
	if err != nil {
		return trace.Wrap(err)
	}
	defer chw.Close()
	auditFile, err := openFD(5, "audit")
	if err != nil {
		return trace.Wrap(err)
	}
	defer auditFile.Close()

	// Ensure the parent process will receive log messages from us
	logger := slog.With(teleport.ComponentKey, teleport.ComponentSubsystemSFTP)

	currentUser, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = os.Stat(currentUser.HomeDir)
	if err != nil {
		return trace.Wrap(err)
	}

	// Read the file transfer request for this session if one exists
	bufferedReader := bufio.NewReader(chr)
	var encodedReq []byte
	var fileTransferReq *srv.FileTransferRequest
	for {
		b, err := bufferedReader.ReadByte()
		if err != nil {
			return trace.Wrap(err)
		}
		// the encoded request will end with a null byte
		if b == 0x0 {
			break
		}
		encodedReq = append(encodedReq, b)
	}
	if len(encodedReq) != 0 {
		fileTransferReq = new(srv.FileTransferRequest)
		if err := json.Unmarshal(encodedReq, fileTransferReq); err != nil {
			return trace.Wrap(err)
		}
	}
	ch := compositeCh{io.NopCloser(bufferedReader), chw}

	sftpEvents := make(chan apievents.AuditEvent, 1)
	h, err := newSFTPHandler(logger, fileTransferReq, sftpEvents)
	if err != nil {
		return trace.Wrap(err)
	}
	handler := sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}
	sftpSrv := sftp.NewRequestServer(ch, handler, sftp.WithStartDirectory(currentUser.HomeDir))

	ctx := context.TODO()
	// Start a goroutine to marshal and send audit events to the parent
	// process to avoid blocking the SFTP connection on event handling
	done := make(chan struct{})
	go func() {
		var m jsonpb.Marshaler
		var buf bytes.Buffer
		for event := range sftpEvents {
			oneOfEvent, err := apievents.ToOneOf(event)
			if err != nil {
				logger.WarnContext(ctx, "Failed to convert SFTP event to OneOf", "error", err)
				continue
			}

			buf.Reset()
			if err := m.Marshal(&buf, oneOfEvent); err != nil {
				logger.WarnContext(ctx, "Failed to marshal SFTP event", "error", err)
				continue
			}

			// Append a NULL byte so the parent process will know where
			// this event ends
			buf.WriteByte(0x0)
			_, err = io.Copy(auditFile, &buf)
			if err != nil {
				logger.WarnContext(ctx, "Failed to send SFTP event to parent", "error", err)
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

	// Send a summary event last
	summaryEvent := &apievents.SFTPSummary{
		Metadata: apievents.Metadata{
			Type: events.SFTPSummaryEvent,
			Code: events.SFTPSummaryCode,
			Time: time.Now(),
		},
	}
	// We don't need to worry about closing these files, handler will
	// take care of that for us
	for _, f := range h.files {
		summaryEvent.FileTransferStats = append(summaryEvent.FileTransferStats, &apievents.FileTransferStat{
			Path:         f.Name(),
			BytesRead:    f.BytesRead(),
			BytesWritten: f.BytesWritten(),
		})
	}
	sftpEvents <- summaryEvent

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
