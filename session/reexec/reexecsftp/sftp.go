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

package reexecsftp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport/session/sftputils"
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

	events chan<- sftputils.Event
}

func newSFTPHandler(logger *slog.Logger, req *FileTransferRequest, events chan<- sftputils.Event) (*sftpHandler, error) {
	var allowed *allowedOps
	if req != nil {
		allowed = &allowedOps{
			write: !req.Download,
		}
		allowedPath, err := sftputils.ExpandHomeDir(filepath.ToSlash(req.Location))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !path.IsAbs(allowedPath) {
			return nil, trace.BadParameter("allowed path must be absolute")
		}
		allowed.path = path.Clean(allowedPath)
	}

	return &sftpHandler{
		logger:  logger,
		allowed: allowed,
		events:  events,
	}, nil
}

// ensureReqIsAllowed returns an error if the SFTP request isn't
// allowed based on the approved file transfer request for this session.
func (s *sftpHandler) ensureReqIsAllowed(req *sftp.Request) error {
	// no specifically allowed operations, all requests are allowed
	if s.allowed == nil {
		return nil
	}

	isDir, err := pathIsDir(req.Filepath)
	if err != nil {
		return trace.Wrap(err)
	}
	if isDir {
		return trace.Errorf("destination path %s is a directory, it must be a file", req.Filepath)
	}

	cleaned := path.Clean(filepath.ToSlash(req.Filepath))
	if s.allowed.path != cleaned {
		return trace.Errorf("operations are only allowed on %s, not %s", s.allowed.path, cleaned)
	}

	switch req.Method {
	case sftputils.MethodLstat, sftputils.MethodStat:
		// these methods are allowed
	case sftputils.MethodGet:
		// only allow reads for downloads
		if s.allowed.write {
			return trace.Errorf("reading is not allowed for this request")
		}
	case sftputils.MethodPut, sftputils.MethodSetStat:
		// only allow writes and chmods for uploads
		if !s.allowed.write {
			return trace.Errorf("writing is not allowed for this request")
		}
	default:
		return trace.Errorf("method %s is not allowed on %s", strings.ToLower(req.Method), req.Filepath)
	}

	return nil
}

func pathIsDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}

	return fi.IsDir(), nil
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
	flags := sftputils.ParseFlags(req)
	var f *os.File
	var err error
	if s.allowed != nil {
		// Files in moderated sessions may not include symlinks.
		f, err = openFileNoFollow(req.Filepath, flags, 0o644)
	} else {
		f, err = os.OpenFile(req.Filepath, flags, 0o644)
	}
	if err != nil {
		// Symlink traversal is not allowed for moderated file transfers.
		if s.allowed != nil && errors.Is(err, syscall.ELOOP) {
			return nil, trace.Errorf("following symlinks is not allowed for moderated file transfers, request the resolved path instead")
		}
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

	if s.allowed != nil && req.Method == sftputils.MethodSetStat {
		// Setstat can be called during moderated file transfers, don't follow
		// symlinks if that's the case.
		return setstatNoFollow(req.Filepath, req.AttrFlags(), req.Attributes())
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
	return sftputils.Realpath(path)
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
	s.events <- sftputils.Event{SFTP: event}
}

func RunSFTP(logger *slog.Logger) error {
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
	var fileTransferReq *FileTransferRequest
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
		fileTransferReq = new(FileTransferRequest)
		if err := json.Unmarshal(encodedReq, fileTransferReq); err != nil {
			return trace.Wrap(err)
		}
	}
	ch := compositeCh{io.NopCloser(bufferedReader), chw}

	sftpEvents := make(chan sftputils.Event, 1)
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
		enc := json.NewEncoder(auditFile)
		enc.SetEscapeHTML(false)
		for event := range sftpEvents {
			err := enc.Encode(event)
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
	summaryEvent := &sftputils.SFTPSummaryEvent{
		Time:  time.Now().UnixNano(),
		Stats: make([]sftputils.SFTPSummaryEventFileTransferStat, 0, len(h.files)),
	}
	// We don't need to worry about closing these files, handler will
	// take care of that for us
	for _, f := range h.files {
		summaryEvent.Stats = append(summaryEvent.Stats, sftputils.SFTPSummaryEventFileTransferStat{
			Path:    f.Name(),
			Read:    f.BytesRead(),
			Written: f.BytesWritten(),
		})
	}
	sftpEvents <- sftputils.Event{Summary: summaryEvent}

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

// openAtAndClose opens a file under the parent directory without following
// symlinks, then closes the parent file. The parent file will be
// closed even if this function returns an error.
func openAtAndClose(parent *os.File, name string, flags int, mode os.FileMode) (*os.File, error) {
	defer parent.Close()
	syscallConn, err := parent.SyscallConn()
	if err != nil {
		return nil, err
	}
	var childFd int
	var openAtErr error
	ctrlErr := syscallConn.Control(func(fd uintptr) {
		for {
			childFd, openAtErr = unix.Openat(int(fd), name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode))
			if !errors.Is(openAtErr, syscall.EINTR) {
				return
			}
		}
	})
	if ctrlErr != nil {
		return nil, ctrlErr
	} else if openAtErr != nil {
		return nil, openAtErr
	}
	return os.NewFile(uintptr(childFd), filepath.Join(parent.Name(), name)), nil
}

// openFileNoFollow opens a file without following symlinks in any part of the path.
func openFileNoFollow(file string, flags int, mode os.FileMode) (*os.File, error) {
	if !filepath.IsAbs(file) {
		return nil, trace.BadParameter("file path must be absolute")
	}
	dir, filename := filepath.Split(file)
	relDir, err := filepath.Rel(string(os.PathSeparator), dir)
	if err != nil {
		return nil, err
	}
	parent, err := os.OpenFile(string(os.PathSeparator), readOnlyPath, 0)
	if err != nil {
		return nil, err
	}
	// Open each directory one at a time to ensure no symlinks are followed.
	for relDir != "" {
		var part string
		part, relDir, _ = strings.Cut(relDir, string(os.PathSeparator))
		parent, err = openAtAndClose(parent, part, unix.O_DIRECTORY|readOnlyPath, 0)
		if err != nil {
			return nil, err
		}
	}
	// Set nonblock so we don't hang in case file is a pipe.
	f, err := openAtAndClose(parent, filename, flags|unix.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, trace.BadParameter("path does not point to a regular file")
	}
	return f, nil
}

func chtimes(file *os.File, atime, mtime time.Time) error {
	syscallConn, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var modifyErr error
	ctrlErr := syscallConn.Control(func(fd uintptr) {
		for {
			modifyErr = unix.Futimes(int(fd), []unix.Timeval{
				unix.NsecToTimeval(atime.UnixNano()),
				unix.NsecToTimeval(mtime.UnixNano()),
			})
			if !errors.Is(modifyErr, syscall.EINTR) {
				return
			}
		}
	})
	if ctrlErr != nil {
		return ctrlErr
	}
	return modifyErr
}

// setstatNoFollow sets file attributes on a file without following any symlinks.
func setstatNoFollow(file string, attrFlags sftp.FileAttrFlags, attrs *sftp.FileStat) error {
	if !attrFlags.Acmodtime && !attrFlags.Permissions && !attrFlags.Size && !attrFlags.UidGid {
		return nil
	}
	mode := os.O_RDONLY
	if attrFlags.Size {
		// Only open in write mode if needed to truncate.
		mode = os.O_WRONLY
	}
	f, err := openFileNoFollow(file, mode, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if attrFlags.Size {
		if err := f.Truncate(int64(attrs.Size)); err != nil {
			return err
		}
	}
	if attrFlags.Acmodtime {
		if err := chtimes(f, time.Unix(int64(attrs.Atime), 0), time.Unix(int64(attrs.Mtime), 0)); err != nil {
			return err
		}
	}
	if attrFlags.Permissions {
		if err := f.Chmod(attrs.FileMode()); err != nil {
			return err
		}
	}
	if attrFlags.UidGid {
		if err := f.Chown(int(attrs.UID), int(attrs.GID)); err != nil {
			return err
		}
	}
	return nil
}
