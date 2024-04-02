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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	methodGet      = "Get"
	methodPut      = "Put"
	methodOpen     = "Open"
	methodSetStat  = "Setstat"
	methodRename   = "Rename"
	methodRmdir    = "Rmdir"
	methodMkdir    = "Mkdir"
	methodLink     = "Link"
	methodSymlink  = "Symlink"
	methodRemove   = "Remove"
	methodList     = "List"
	methodStat     = "Stat"
	methodLstat    = "Lstat"
	methodReadlink = "Readlink"
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
	logger  *log.Entry
	allowed *allowedOps
	events  chan<- *apievents.SFTP
}

func newSFTPHandler(logger *log.Entry, req *srv.FileTransferRequest, events chan<- *apievents.SFTP) (*sftpHandler, error) {
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
	case methodLstat, methodStat:
		// these methods are allowed
	case methodGet:
		// only allow reads for downloads
		if s.allowed.write {
			return newDisallowedErr(req)
		}
	case methodPut, methodSetStat:
		// only allow writes and chmods for uploads
		if !s.allowed.write {
			return newDisallowedErr(req)
		}
	default:
		return newDisallowedErr(req)
	}

	return nil
}

// OpenFile handles 'open' requests when opening a file for reading
// and writing is desired.
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

func (s *sftpHandler) openFile(req *sftp.Request) (*os.File, error) {
	if err := s.ensureReqIsAllowed(req); err != nil {
		return nil, err
	}

	var flags int
	pflags := req.Pflags()
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

	if pflags.Read && pflags.Write {
		flags |= os.O_RDWR
	} else if pflags.Read {
		flags |= os.O_RDONLY
	} else if pflags.Write {
		flags |= os.O_WRONLY
	}

	f, err := os.OpenFile(req.Filepath, flags, defaults.FilePermissions)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// Filecmd handles file modification requests.
func (s *sftpHandler) Filecmd(req *sftp.Request) (retErr error) {
	defer func() {
		if retErr == sftp.ErrSSHFxOpUnsupported {
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

	switch req.Method {
	case methodSetStat:
		return s.setstat(req)
	case methodRename:
		if req.Target == "" {
			return os.ErrInvalid
		}
		return os.Rename(req.Filepath, req.Target)
	case methodRmdir:
		fi, err := os.Lstat(req.Filepath)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return fmt.Errorf("%q is not a directory", req.Filepath)
		}
		return os.RemoveAll(req.Filepath)
	case methodMkdir:
		return os.MkdirAll(req.Filepath, defaults.DirectoryPermissions)
	case methodLink:
		if req.Target == "" {
			return os.ErrInvalid
		}
		return os.Link(req.Target, req.Filepath)
	case methodSymlink:
		if req.Target == "" {
			return os.ErrInvalid
		}
		return os.Symlink(req.Target, req.Filepath)
	case methodRemove:
		fi, err := os.Lstat(req.Filepath)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return fmt.Errorf("%q is a directory", req.Filepath)
		}
		return os.Remove(req.Filepath)
	default:
		return sftp.ErrSSHFxOpUnsupported
	}
}

func (s *sftpHandler) setstat(req *sftp.Request) error {
	attrFlags := req.AttrFlags()
	attrs := req.Attributes()

	if attrFlags.Acmodtime {
		atime := time.Unix(int64(attrs.Atime), 0)
		mtime := time.Unix(int64(attrs.Mtime), 0)

		err := os.Chtimes(req.Filepath, atime, mtime)
		if err != nil {
			return err
		}
	}
	if attrFlags.Permissions {
		err := os.Chmod(req.Filepath, attrs.FileMode())
		if err != nil {
			return err
		}
	}
	if attrFlags.UidGid {
		err := os.Chown(req.Filepath, int(attrs.UID), int(attrs.GID))
		if err != nil {
			return err
		}
	}
	if attrFlags.Size {
		err := os.Truncate(req.Filepath, int64(attrs.Size))
		if err != nil {
			return err
		}
	}

	return nil
}

// listerAt satisfies [sftp.ListerAt].
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

// Filelist handles 'readdir', 'stat' and 'readlink' requests.
func (s *sftpHandler) Filelist(req *sftp.Request) (_ sftp.ListerAt, retErr error) {
	defer func() {
		if req.Method == methodList {
			s.sendSFTPEvent(req, retErr)
		}
	}()

	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if err := s.ensureReqIsAllowed(req); err != nil {
		return nil, err
	}

	switch req.Method {
	case methodList:
		entries, err := os.ReadDir(req.Filepath)
		if err != nil {
			return nil, err
		}
		infos := make([]fs.FileInfo, len(entries))
		for i, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			infos[i] = info
		}
		return listerAt(infos), nil
	case methodStat:
		fi, err := os.Stat(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerAt{fi}, nil
	case methodReadlink:
		dst, err := os.Readlink(req.Filepath)
		if err != nil {
			return nil, err
		}
		return listerAt{fileName(dst)}, nil
	default:
		return nil, sftp.ErrSSHFxOpUnsupported
	}
}

// Lstat handles 'lstat' requests.
func (s *sftpHandler) Lstat(req *sftp.Request) (sftp.ListerAt, error) {
	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if err := s.ensureReqIsAllowed(req); err != nil {
		return nil, err
	}

	fi, err := os.Lstat(req.Filepath)
	if err != nil {
		return nil, err
	}

	return listerAt{fi}, nil
}

func (s *sftpHandler) sendSFTPEvent(req *sftp.Request, reqErr error) {
	event := &apievents.SFTP{
		Metadata: apievents.Metadata{
			Type: events.SFTPEvent,
			Time: time.Now(),
		},
	}

	switch req.Method {
	case methodOpen, methodGet, methodPut:
		if reqErr == nil {
			event.Code = events.SFTPOpenCode
		} else {
			event.Code = events.SFTPOpenFailureCode
		}
		event.Action = apievents.SFTPAction_OPEN
	case methodSetStat:
		if reqErr == nil {
			event.Code = events.SFTPSetstatCode
		} else {
			event.Code = events.SFTPSetstatFailureCode
		}
		event.Action = apievents.SFTPAction_SETSTAT
	case methodList:
		if reqErr == nil {
			event.Code = events.SFTPReaddirCode
		} else {
			event.Code = events.SFTPReaddirFailureCode
		}
		event.Action = apievents.SFTPAction_READDIR
	case methodRemove:
		if reqErr == nil {
			event.Code = events.SFTPRemoveCode
		} else {
			event.Code = events.SFTPRemoveFailureCode
		}
		event.Action = apievents.SFTPAction_REMOVE
	case methodMkdir:
		if reqErr == nil {
			event.Code = events.SFTPMkdirCode
		} else {
			event.Code = events.SFTPMkdirFailureCode
		}
		event.Action = apievents.SFTPAction_MKDIR
	case methodRmdir:
		if reqErr == nil {
			event.Code = events.SFTPRmdirCode
		} else {
			event.Code = events.SFTPRmdirFailureCode
		}
		event.Action = apievents.SFTPAction_RMDIR
	case methodRename:
		if reqErr == nil {
			event.Code = events.SFTPRenameCode
		} else {
			event.Code = events.SFTPRenameFailureCode
		}
		event.Action = apievents.SFTPAction_RENAME
	case methodSymlink:
		if reqErr == nil {
			event.Code = events.SFTPSymlinkCode
		} else {
			event.Code = events.SFTPSymlinkFailureCode
		}
		event.Action = apievents.SFTPAction_SYMLINK
	case methodLink:
		if reqErr == nil {
			event.Code = events.SFTPLinkCode
		} else {
			event.Code = events.SFTPLinkFailureCode
		}
		event.Action = apievents.SFTPAction_LINK
	default:
		s.logger.Warnf("Unknown SFTP request %q", req.Method)
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get working dir.")
	}

	event.WorkingDirectory = wd
	event.Path = req.Filepath
	event.TargetPath = req.Target
	event.Flags = req.Flags
	if req.Method == methodSetStat {
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
		s.logger.Debugf("%s: %v", req.Method, reqErr)
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
	l := utils.NewLogger()
	logger := l.WithField(teleport.ComponentKey, teleport.ComponentSubsystemSFTP)

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

	sftpEvents := make(chan *apievents.SFTP, 1)
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

	// Start a goroutine to marshal and send audit events to the parent
	// process to avoid blocking the SFTP connection on event handling
	done := make(chan struct{})
	go func() {
		var m jsonpb.Marshaler
		var buf bytes.Buffer
		for event := range sftpEvents {
			buf.Reset()
			if err := m.Marshal(&buf, event); err != nil {
				logger.WithError(err).Warn("Failed to marshal SFTP event.")
			} else {
				// Append a NULL byte so the parent process will know where
				// this event ends
				buf.WriteByte(0x0)
				_, err = io.Copy(auditFile, &buf)
				if err != nil {
					logger.WithError(err).Warn("Failed to send SFTP event to parent.")
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
