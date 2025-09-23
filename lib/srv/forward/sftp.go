// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package forward

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
	sftputils "github.com/gravitational/teleport/lib/sshutils/sftp"
)

// SFTPProxy proxies an SFTP session and emits audit events for the handled
// commands.
type SFTPProxy struct {
	srv      *sftp.RequestServer
	handlers *proxyHandlers
}

// NewSFTPProxy creates a new SFTPProxy to serve the given channel.
func NewSFTPProxy(
	scx *srv.ServerContext,
	channel io.ReadWriteCloser,
	logger *slog.Logger,
) (*SFTPProxy, error) {
	if scx == nil {
		return nil, trace.BadParameter("missing parameter scx")
	}
	if channel == nil {
		return nil, trace.BadParameter("missing parameter channel")
	}
	if logger == nil {
		logger = slog.With(teleport.ComponentKey, "SFTP")
	}

	client, err := sftp.NewClient(scx.RemoteClient.Client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteFS := sftputils.NewRemoteFilesystem(client)
	wd, err := remoteFS.Getwd()
	if err != nil {
		logger.WarnContext(scx.CancelContext(), `Unable to get working directory, defaulting to "/"`)
	}
	h := &proxyHandlers{
		scx:      scx,
		remoteFS: remoteFS,
		logger:   logger,
	}
	handlers := sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}
	srv := sftp.NewRequestServer(channel, handlers, sftp.WithStartDirectory(wd))
	return &SFTPProxy{srv: srv, handlers: h}, nil
}

// Serve serves SFTP requests. It returns after either [*SFTPProxy.Close] is
// called or the underlying connection is closed.
func (p *SFTPProxy) Serve() error {
	// Run server to completion.
	serveErr := p.srv.Serve()
	// After the server has finished, send a summary event.
	scx := p.handlers.scx
	summaryEvent := &apievents.SFTPSummary{
		Metadata: apievents.Metadata{
			Type: events.SFTPSummaryEvent,
			Code: events.SFTPSummaryCode,
			Time: time.Now(),
		},
		ServerMetadata:  scx.GetServer().EventMetadata(),
		SessionMetadata: scx.GetSessionMetadata(),
		UserMetadata:    scx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: scx.ServerConn.RemoteAddr().String(),
			LocalAddr:  scx.ServerConn.LocalAddr().String(),
		},
	}

	for _, f := range p.handlers.files {
		summaryEvent.FileTransferStats = append(summaryEvent.FileTransferStats, &apievents.FileTransferStat{
			Path:         f.Name(),
			BytesRead:    f.BytesRead(),
			BytesWritten: f.BytesWritten(),
		})
	}
	if err := scx.GetServer().EmitAuditEvent(scx.CancelContext(), summaryEvent); err != nil {
		p.handlers.logger.WarnContext(scx.CancelContext(), "Failed to emit SFTP summary event", "error", err)
	}

	return trace.Wrap(serveErr)
}

// Close closes the SFTP proxy.
func (p *SFTPProxy) Close() error {
	return trace.Wrap(p.srv.Close())
}

type proxyHandlers struct {
	scx      *srv.ServerContext
	remoteFS sftputils.FileSystem
	logger   *slog.Logger

	fileMtx sync.Mutex
	files   []*sftputils.TrackedFile
}

// Fileread handles Open requests for reading files.
func (h *proxyHandlers) Fileread(req *sftp.Request) (_ io.ReaderAt, err error) {
	defer h.sendSFTPEvent(req, err)
	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if !req.Pflags().Read {
		return nil, os.ErrInvalid
	}
	f, err := h.remoteFS.Open(req.Filepath)
	if err != nil {
		return nil, err
	}
	return h.trackFile(f), nil
}

// Filewrite handles Open requests for writing files.
func (h *proxyHandlers) Filewrite(req *sftp.Request) (_ io.WriterAt, err error) {
	defer h.sendSFTPEvent(req, err)
	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if !req.Pflags().Write {
		return nil, os.ErrInvalid
	}
	f, err := h.remoteFS.Create(req.Filepath, 0)
	if err != nil {
		return nil, err
	}
	return h.trackFile(f), nil
}

// OpenFile handles Open requests for both reading and writing. Required to
// satisfy [sftp.OpenFileWriter].
func (h *proxyHandlers) OpenFile(req *sftp.Request) (_ sftp.WriterAtReaderAt, retErr error) {
	defer h.sendSFTPEvent(req, retErr)

	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}

	f, err := h.remoteFS.OpenFile(req.Filepath, sftputils.ParseFlags(req))
	if err != nil {
		return nil, err
	}
	return h.trackFile(f), nil
}

func (h *proxyHandlers) trackFile(f sftputils.File) sftp.WriterAtReaderAt {
	trackFile := &sftputils.TrackedFile{File: f}
	h.fileMtx.Lock()
	defer h.fileMtx.Unlock()
	h.files = append(h.files, trackFile)
	return trackFile
}

// Filecmd handles file commands.
func (h *proxyHandlers) Filecmd(req *sftp.Request) (err error) {
	defer func() {
		if !errors.Is(err, sftp.ErrSSHFxOpUnsupported) {
			h.sendSFTPEvent(req, err)
		}
	}()
	return sftputils.HandleFilecmd(req, h.remoteFS)
}

// Filelist handles listing info about files.
func (h *proxyHandlers) Filelist(req *sftp.Request) (_ sftp.ListerAt, err error) {
	defer func() {
		if req.Method == sftputils.MethodList {
			h.sendSFTPEvent(req, err)
		}
	}()
	lister, err := sftputils.HandleFilelist(req, h.remoteFS)
	if err != nil {
		return nil, err
	}
	return lister, nil
}

func (h *proxyHandlers) sendSFTPEvent(req *sftp.Request, reqErr error) {
	wd, err := h.remoteFS.Getwd()
	if err != nil {
		h.logger.WarnContext(req.Context(), "Unable to get working directory", "error", err)
		// Emit event without working directory.
	}
	event, err := sftputils.ParseSFTPEvent(req, wd, reqErr)
	if err != nil {
		h.logger.WarnContext(req.Context(), "Unknown SFTP request", "request", req.Method)
		return
	} else if reqErr != nil {
		h.logger.DebugContext(req.Context(), "failed handling SFTP request", "request", req.Method, "error", reqErr)
	}
	event.ServerMetadata = h.scx.GetServer().EventMetadata()
	event.SessionMetadata = h.scx.GetSessionMetadata()
	event.UserMetadata = h.scx.Identity.GetUserMetadata()
	event.ConnectionMetadata = apievents.ConnectionMetadata{
		RemoteAddr: h.scx.ServerConn.RemoteAddr().String(),
		LocalAddr:  h.scx.ServerConn.LocalAddr().String(),
	}
	if err := h.scx.GetServer().EmitAuditEvent(req.Context(), event); err != nil {
		h.logger.WarnContext(req.Context(), "Failed to emit SFTP event", "error", err)
	}
}
