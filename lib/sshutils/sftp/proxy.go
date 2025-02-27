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

package sftp

import (
	"context"
	"io"
	"sync"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPProxy struct {
	srv *sftp.RequestServer
}

func NewSFTPProxy(ctx context.Context, conn io.ReadWriteCloser, clt *ssh.Client) (*SFTPProxy, error) {
	client, err := sftp.NewClient(clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h := &proxyHandlers{
		ctx:      ctx,
		remoteFS: &remoteFS{c: client},
	}
	handlers := sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}
	srv := sftp.NewRequestServer(conn, handlers)
	return &SFTPProxy{srv: srv}, nil
}

func (p *SFTPProxy) Serve() error {
	return trace.Wrap(p.srv.Serve())
}

func (p *SFTPProxy) Close() error {
	return trace.Wrap(p.srv.Close())
}

type proxyHandlers struct {
	ctx      context.Context
	remoteFS FileSystem
	files    []*TrackedFile
	mtx      sync.Mutex
}

func (h *proxyHandlers) Fileread(req *sftp.Request) (io.ReaderAt, error) {
	f, err := h.remoteFS.Open(req.Context(), req.Filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return h.trackFile(f), nil
}

func (h *proxyHandlers) Filewrite(req *sftp.Request) (io.WriterAt, error) {
	f, err := h.remoteFS.Create(req.Context(), req.Filepath, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return h.trackFile(f), trace.Wrap(err)
}

func (h *proxyHandlers) trackFile(f File) sftp.WriterAtReaderAt {
	trackFile := &TrackedFile{File: f}
	h.mtx.Lock()
	h.files = append(h.files, trackFile)
	h.mtx.Unlock()
	return trackFile
}

func (h *proxyHandlers) Filecmd(req *sftp.Request) error {
	return trace.Wrap(HandleFilecmd(req, h.remoteFS))
}

func (h *proxyHandlers) Filelist(req *sftp.Request) (sftp.ListerAt, error) {
	lister, err := HandleFilelist(req, h.remoteFS)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return lister, nil
}
