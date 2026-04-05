/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package forward

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	sftputils "github.com/gravitational/teleport/lib/sshutils/sftp"
)

func TestSFTPProxyServeClosesRemoteFilesystem(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("remote filesystem close failed")
	remoteFS := &closeTrackingFS{closeErr: closeErr}

	proxy := newTestSFTPProxy(remoteFS)

	err := proxy.Serve()
	require.Error(t, err)
	require.True(t, remoteFS.closed, "Serve should close the remote filesystem")
}

func newTestSFTPProxy(remoteFS sftputils.FileSystem) *SFTPProxy {
	handlers := &proxyHandlers{
		auditContext: mockSFTPAuditContext{},
		remoteFS:     remoteFS,
		logger:       slog.Default(),
	}
	srv := sftp.NewRequestServer(&eofReadWriteCloser{}, sftp.Handlers{
		FileGet:  handlers,
		FilePut:  handlers,
		FileCmd:  handlers,
		FileList: handlers,
	}, sftp.WithStartDirectory("/"))

	return &SFTPProxy{srv: srv, handlers: handlers}
}

type mockSFTPAuditContext struct{}

func (mockSFTPAuditContext) CancelContext() context.Context {
	return context.Background()
}

func (mockSFTPAuditContext) EmitAuditEvent(context.Context, apievents.AuditEvent) error {
	return nil
}

func (mockSFTPAuditContext) ServerMetadata() apievents.ServerMetadata {
	return apievents.ServerMetadata{}
}

func (mockSFTPAuditContext) GetSessionMetadata() apievents.SessionMetadata {
	return apievents.SessionMetadata{}
}

func (mockSFTPAuditContext) UserMetadata() apievents.UserMetadata {
	return apievents.UserMetadata{}
}

func (mockSFTPAuditContext) ConnectionMetadata() apievents.ConnectionMetadata {
	return apievents.ConnectionMetadata{}
}

type closeTrackingFS struct {
	sftputils.FileSystem
	closed   bool
	closeErr error
}

func (f *closeTrackingFS) Close() error {
	f.closed = true
	return f.closeErr
}

type eofReadWriteCloser struct{}

func (e *eofReadWriteCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (e *eofReadWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (e *eofReadWriteCloser) Close() error {
	return nil
}
