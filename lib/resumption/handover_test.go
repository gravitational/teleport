// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package resumption

import (
	"context"
	"encoding/binary"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestHandover(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	sshServer := discardingSSHServer(t)
	hostID := uuid.NewString()
	// unix domain socket names have a very tight length limit
	dataDir := t.TempDir()

	s1 := NewSSHServerWrapper(SSHServerWrapperConfig{
		SSHServer: sshServer,
		HostID:    hostID,
		DataDir:   dataDir,
	})
	s2 := NewSSHServerWrapper(SSHServerWrapperConfig{
		SSHServer: sshServer,
		HostID:    hostID,
		DataDir:   dataDir,
	})

	dial := func(handleConnection func(net.Conn), clientAddr netip.Addr) net.Conn {
		c1, c2, err := uds.NewSocketpair(uds.SocketTypeStream)
		require.NoError(err)

		srv := &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 1 + rand.Intn(65535),
		}
		clt := &net.TCPAddr{
			IP:   clientAddr.AsSlice(),
			Zone: clientAddr.Zone(),
			Port: 1 + rand.Intn(65535),
		}

		go handleConnection(utils.NewConnWithAddr(c2, srv, clt))
		conn := utils.NewConnWithAddr(c1, clt, srv)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}

	originalNC := dial(s1.HandleConnection, netip.MustParseAddr("127.0.0.1"))

	redialConns := make(chan net.Conn)
	defer close(redialConns)

	wrappedNC, err := WrapSSHClientConn(context.Background(), originalNC, func(ctx context.Context, receivedHostID string) (net.Conn, error) {
		if receivedHostID != hostID {
			return nil, trace.BadParameter("expected hostID %q, got %q", hostID, receivedHostID)
		}
		conn := <-redialConns
		if conn == nil {
			return nil, trace.ConnectionProblem(nil, "no redial connection received")
		}
		return conn, nil
	})
	require.NoError(err)
	defer wrappedNC.Close()

	require.IsType((*Conn)(nil), wrappedNC)

	clt, err := sshClient(wrappedNC)
	require.NoError(err)
	defer clt.Close()

	const wantReplyTrue = true
	_, _, err = clt.SendRequest("foo", wantReplyTrue, nil)
	require.NoError(err)

	_ = originalNC.Close()
	nextNC := dial(s2.HandleConnection, netip.MustParseAddr("127.0.0.1"))
	redialConns <- nextNC

	_, _, err = clt.SendRequest("foo", wantReplyTrue, nil)
	require.NoError(err)

	_ = nextNC.Close()
	// this will result in a closed connection, because changing network address
	// stops further reconnection attempts
	redialConns <- dial(s2.HandleConnection, netip.MustParseAddr("127.0.0.2"))

	require.ErrorIs(clt.Wait(), net.ErrClosed)
}

func TestHandoverCleanup(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	hostID := uuid.NewString()
	dataDir := t.TempDir()

	var tok resumptionToken
	binary.NativeEndian.PutUint64(tok[:8], rand.Uint64())
	binary.NativeEndian.PutUint64(tok[8:], rand.Uint64())

	s := NewSSHServerWrapper(SSHServerWrapperConfig{
		SSHServer: func(c net.Conn) {
			defer c.Close()
			assert.Fail(t, "unexpected connection")
		},
		HostID:  hostID,
		DataDir: dataDir,
	})

	handoverDir := filepath.Join(dataDir, "handover")
	require.NoError(os.MkdirAll(handoverDir, teleport.PrivateDirMode))

	d, err := os.ReadDir(handoverDir)
	require.NoError(err)
	require.Empty(d)

	l, err := uds.ListenUnix(context.Background(), "unix", sockPath(dataDir, tok))
	require.NoError(err)
	l.SetUnlinkOnClose(false)
	defer l.Close()
	go func() {
		defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
				break
			}
			_ = c.Close()
		}
	}()

	d, err = os.ReadDir(handoverDir)
	require.NoError(err)
	require.NotEmpty(d)

	ctx := context.Background()

	const cleanupDelayZero time.Duration = 0
	require.NoError(s.handoverCleanup(ctx, cleanupDelayZero))

	d, err = os.ReadDir(handoverDir)
	require.NoError(err)
	require.NotEmpty(d)

	l.Close()
	require.NoError(s.handoverCleanup(ctx, cleanupDelayZero))

	d, err = os.ReadDir(handoverDir)
	require.NoError(err)
	require.Empty(d)
}
