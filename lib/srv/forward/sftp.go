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
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/pkg/sftp"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv"
	sftputils "github.com/gravitational/teleport/lib/sshutils/sftp"
)

type SFTPProxy struct {
	srv      *sftp.RequestServer
	handlers *proxyHandlers
}

func NewSFTPProxy(
	scx *srv.ServerContext,
	channel io.ReadWriteCloser,
) (*SFTPProxy, error) {
	client, err := sftp.NewClient(scx.RemoteClient.Client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h := &proxyHandlers{
		scx:      scx,
		remoteFS: sftputils.NewRemoteFilesystem(client),
		logger:   slog.With(teleport.ComponentKey, teleport.ComponentSubsystemSFTP),
	}
	handlers := sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}
	srv := sftp.NewRequestServer(channel, handlers)
	return &SFTPProxy{srv: srv, handlers: h}, nil
}

func (p *SFTPProxy) Serve() error {
	return trace.Wrap(p.srv.Serve())
}

func (p *SFTPProxy) Close() error {
	if err := p.srv.Close(); err != nil || !errors.Is(err, io.EOF) {
		return trace.Wrap(err)
	}
	// Send a summary event last
	scx := p.handlers.scx
	summaryEvent := &apievents.SFTPSummary{
		Metadata: apievents.Metadata{
			Type: events.SFTPSummaryEvent,
			Code: events.SFTPSummaryCode,
			Time: time.Now(),
		},
		ServerMetadata:  scx.GetServer().TargetMetadata(),
		SessionMetadata: scx.GetSessionMetadata(),
		UserMetadata:    scx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: scx.ServerConn.RemoteAddr().String(),
			LocalAddr:  scx.ServerConn.LocalAddr().String(),
		},
	}

	for _, f := range p.handlers.files {
		summaryEvent.FileTransferStats = append(summaryEvent.FileTransferStats, &apievents.FileTransferStat{
			Path:         f.File.Name(),
			BytesRead:    f.BytesRead.Load(),
			BytesWritten: f.BytesWritten.Load(),
		})
	}
	if err := scx.GetServer().EmitAuditEvent(scx.CancelContext(), summaryEvent); err != nil {
		p.handlers.logger.WarnContext(scx.CancelContext(), "Failed to emit SFTP summary event", "error", err)
	}
	return nil
}

type proxyHandlers struct {
	ctx      context.Context
	scx      *srv.ServerContext
	remoteFS sftputils.FileSystem
	files    []*sftputils.TrackedFile
	mtx      sync.Mutex
	logger   *slog.Logger
}

func (h *proxyHandlers) Fileread(req *sftp.Request) (_ io.ReaderAt, err error) {
	defer h.sendSFTPEvent(req, err)
	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if !req.Pflags().Read {
		return nil, os.ErrInvalid
	}
	f, err := h.remoteFS.Open(req.Context(), req.Filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return h.trackFile(f), nil
}

func (h *proxyHandlers) Filewrite(req *sftp.Request) (_ io.WriterAt, err error) {
	defer h.sendSFTPEvent(req, err)
	if req.Filepath == "" {
		return nil, os.ErrInvalid
	}
	if !req.Pflags().Write {
		return nil, os.ErrInvalid
	}
	f, err := h.remoteFS.Create(req.Context(), req.Filepath, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return h.trackFile(f), trace.Wrap(err)
}

func (h *proxyHandlers) trackFile(f sftputils.File) sftp.WriterAtReaderAt {
	trackFile := &sftputils.TrackedFile{File: f}
	h.mtx.Lock()
	h.files = append(h.files, trackFile)
	h.mtx.Unlock()
	return trackFile
}

func (h *proxyHandlers) Filecmd(req *sftp.Request) (err error) {
	defer func() {
		if errors.Is(err, sftp.ErrSSHFxOpUnsupported) {
			return
		}
		h.sendSFTPEvent(req, err)
	}()
	return trace.Wrap(sftputils.HandleFilecmd(req, h.remoteFS))
}

func (h *proxyHandlers) Filelist(req *sftp.Request) (_ sftp.ListerAt, err error) {
	defer func() {
		if req.Method == sftputils.MethodList {
			h.sendSFTPEvent(req, err)
		}
	}()
	lister, err := sftputils.HandleFilelist(req, h.remoteFS)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return lister, nil
}

func (h *proxyHandlers) sendSFTPEvent(req *sftp.Request, reqErr error) {
	event, err := sftputils.ParseSFTPEvent(req, reqErr)
	if err != nil {
		h.logger.WarnContext(req.Context(), "Unknown SFTP request", "request", req.Method)
		return
	} else if reqErr != nil {
		h.logger.DebugContext(req.Context(), "failed handling SFTP request", "request", req.Method, "error", reqErr)
	}
	event.ServerMetadata = h.scx.GetServer().TargetMetadata()
	event.SessionMetadata = h.scx.GetSessionMetadata()
	event.UserMetadata = h.scx.Identity.GetUserMetadata()
	event.ConnectionMetadata = apievents.ConnectionMetadata{
		RemoteAddr: h.scx.ServerConn.RemoteAddr().String(),
		LocalAddr:  h.scx.ServerConn.LocalAddr().String(),
	}
	// TODO: figure out working directory if possible
	if err := h.scx.GetServer().EmitAuditEvent(req.Context(), event); err != nil {
		h.logger.WarnContext(req.Context(), "Failed to emit SFTP event", "error", err)
	}
}
