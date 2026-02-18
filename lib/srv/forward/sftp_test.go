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
	"net"
	"reflect"
	"testing"
	"unsafe"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	sftputils "github.com/gravitational/teleport/lib/sshutils/sftp"
)

func TestSFTPProxyServeClosesRemoteFilesystem(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("remote filesystem close failed")
	remoteFS := &closeTrackingFS{closeErr: closeErr}

	proxy := newTestSFTPProxy(t, remoteFS)

	err := proxy.Serve()
	require.ErrorIs(t, err, closeErr)
	require.True(t, remoteFS.closed, "Serve should close the remote filesystem")
}

func newTestSFTPProxy(t *testing.T, remoteFS sftputils.FileSystem) *SFTPProxy {
	t.Helper()

	server := &Server{
		StreamEmitter: &eventstest.MockRecorderEmitter{},
		targetServer:  &types.ServerV2{},
	}

	scx := &srv.ServerContext{
		ConnectionContext: &sshutils.ConnectionContext{
			ServerConn: &ssh.ServerConn{Conn: &mockSSHConn{
				remoteAddr: &net.TCPAddr{IP: net.ParseIP("10.0.0.2"), Port: 3022},
				localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3023},
			}},
		},
		Identity: srv.IdentityContext{UnmappedIdentity: &sshca.Identity{}},
	}
	setServerContextField(t, scx, "srv", srv.Server(server))
	setServerContextField(t, scx, "cancelContext", context.Background())

	handlers := &proxyHandlers{
		scx:      scx,
		remoteFS: remoteFS,
		logger:   slog.Default(),
	}
	srv := sftp.NewRequestServer(&eofReadWriteCloser{}, sftp.Handlers{
		FileGet:  handlers,
		FilePut:  handlers,
		FileCmd:  handlers,
		FileList: handlers,
	}, sftp.WithStartDirectory("/"))

	return &SFTPProxy{srv: srv, handlers: handlers}
}

func setServerContextField(t *testing.T, scx *srv.ServerContext, field string, value any) {
	t.Helper()
	fv := reflect.ValueOf(scx).Elem().FieldByName(field)
	require.Truef(t, fv.IsValid(), "missing ServerContext field %q", field)
	reflect.NewAt(fv.Type(), unsafe.Pointer(fv.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
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

type mockSSHConn struct {
	remoteAddr net.Addr
	localAddr  net.Addr
}

func (c *mockSSHConn) User() string {
	return ""
}

func (c *mockSSHConn) SessionID() []byte {
	return []byte{1}
}

func (c *mockSSHConn) ClientVersion() []byte {
	return []byte{1}
}

func (c *mockSSHConn) ServerVersion() []byte {
	return []byte{1}
}

func (c *mockSSHConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *mockSSHConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *mockSSHConn) Close() error {
	return nil
}

func (c *mockSSHConn) SendRequest(string, bool, []byte) (bool, []byte, error) {
	return false, nil, nil
}

func (c *mockSSHConn) OpenChannel(string, []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, nil
}

func (c *mockSSHConn) Wait() error {
	return nil
}
