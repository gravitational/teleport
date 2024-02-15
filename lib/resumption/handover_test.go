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
	"math/rand"
	"net"
	"os"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestHandover(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	sshServer := discardingSSHServer(t)
	hostID := uuid.NewString()
	dataDir := shortTempDir(t)
	t.Logf("using temporary data dir %q", dataDir)

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

	dial := func(handleConnection func(net.Conn)) (net.Conn, error) {
		c1, c2, err := uds.NewSocketpair(uds.SocketTypeStream)
		if err != nil {
			return nil, err
		}

		a1 := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: rand.Intn(65536)}
		a2 := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: rand.Intn(65536)}

		go handleConnection(utils.NewConnWithAddr(c2, a2, a1))
		return utils.NewConnWithAddr(c1, a1, a2), nil
	}

	originalNC, err := dial(s1.HandleConnection)
	require.NoError(err)
	defer originalNC.Close()

	redialDestination := make(chan func(net.Conn))
	defer close(redialDestination)

	wrappedNC, err := WrapSSHClientConn(context.Background(), originalNC, func(ctx context.Context, receivedHostID string) (net.Conn, error) {
		if receivedHostID != hostID {
			return nil, trace.BadParameter("expected hostID %q, got %q", hostID, receivedHostID)
		}
		handleConnection := <-redialDestination
		if handleConnection == nil {
			return nil, trace.ConnectionProblem(nil, "no redial destination received")
		}
		return dial(handleConnection)
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
	redialDestination <- s2.HandleConnection

	_, _, err = clt.SendRequest("foo", wantReplyTrue, nil)
	require.NoError(err)
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	base := ""
	if runtime.GOOS == "darwin" {
		base = "/tmp"
	}
	d, err := os.MkdirTemp(base, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(d)) })
	return d
}
