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
	"bufio"
	"context"
	"crypto/ed25519"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestResumption(t *testing.T) {
	hostID := uuid.NewString()

	sshServer := discardingSSHServer(t)
	resumableServer := NewSSHServerWrapper(SSHServerWrapperConfig{
		SSHServer: sshServer,
		HostID:    hostID,
	})

	directListener, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	t.Cleanup(func() { directListener.Close() })

	muxListener, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	t.Cleanup(func() { muxListener.Close() })

	mux, err := multiplexer.New(multiplexer.Config{
		Listener:          muxListener,
		PROXYProtocolMode: multiplexer.PROXYProtocolOff,
		ID:                "testresumption",
		PreDetect:         resumableServer.PreDetect,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mux.Close() })

	muxSSH := mux.SSH()
	go mux.Serve()

	go func() {
		for {
			c, err := directListener.Accept()
			if err != nil {
				return
			}
			go resumableServer.HandleConnection(c)
		}
	}()

	go func() {
		for {
			c, err := muxSSH.Accept()
			if err != nil {
				return
			}
			go sshServer(c)
		}
	}()

	t.Run("mux", func(t *testing.T) {
		t.Parallel()
		testResumption(t, muxSSH.Addr().Network(), muxSSH.Addr().String(), hostID)
	})

	t.Run("tunnel", func(t *testing.T) {
		t.Parallel()
		testResumption(t, directListener.Addr().Network(), directListener.Addr().String(), hostID)
	})

	t.Run("no roaming", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		originalNC, err := net.Dial(directListener.Addr().Network(), directListener.Addr().String())
		require.NoError(err)
		t.Cleanup(func() { originalNC.Close() })

		clock := clockwork.NewFakeClock()
		redialingSyncPoint := make(chan struct{})
		resumableNC, err := wrapSSHClientConn(ctx, originalNC, func(ctx context.Context, newHostID string) (net.Conn, error) {
			if newHostID != hostID {
				return nil, trace.BadParameter("expected hostID %q, got %q", hostID, newHostID)
			}
			<-redialingSyncPoint

			p1, p2, err := uds.NewSocketpair(uds.SocketTypeStream)
			if err != nil {
				return nil, err
			}

			// the original connection is on localhost, which is distincly not 127.0.0.42
			go resumableServer.HandleConnection(
				utils.NewConnWithSrcAddr(p1, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 42), Port: 55555}),
			)

			return p2, nil
		}, clock)
		require.NoError(err)

		var buf [4]byte
		_, err = io.ReadFull(resumableNC, buf[:])
		require.NoError(err)

		originalNC.Close()
		redialingSyncPoint <- struct{}{}

		_, err = io.Copy(io.Discard, resumableNC)
		require.ErrorIs(err, net.ErrClosed)
	})
}

func testResumption(t *testing.T, network, address string, expectedHostID string) {
	t.Run("expecting SSH version identifier", func(t *testing.T) {
		require := require.New(t)

		nc, err := net.Dial(network, address)
		require.NoError(err)
		t.Cleanup(func() { nc.Close() })

		line, err := bufio.NewReader(nc).ReadString('\n')
		require.NoError(err)

		require.Equal("SSH-2.0-", line[:8])
		require.Equal("\r\n", line[len(line)-2:])
	})

	t.Run("plain SSH", func(t *testing.T) {
		require := require.New(t)

		nc, err := net.Dial(network, address)
		require.NoError(err)
		t.Cleanup(func() { nc.Close() })

		clt, err := sshClient(nc)
		require.NoError(err)
		t.Cleanup(func() { clt.Close() })

		require.True(strings.HasPrefix(string(clt.ServerVersion()), "SSH-2.0-Teleport resume-v1 "))
	})

	t.Run("SSH through resumable conn", func(t *testing.T) {
		require := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		originalNC, err := net.Dial(network, address)
		require.NoError(err)
		t.Cleanup(func() { originalNC.Close() })

		clock := clockwork.NewFakeClock()
		redialingSyncPoint := make(chan struct{})
		resumableNC, err := wrapSSHClientConn(ctx, originalNC, func(ctx context.Context, hostID string) (net.Conn, error) {
			if hostID != expectedHostID {
				return nil, trace.BadParameter("expected hostID %q, got %q", expectedHostID, hostID)
			}
			<-redialingSyncPoint
			return net.Dial(network, address)
		}, clock)
		require.NoError(err)
		t.Cleanup(func() { resumableNC.Close() })
		require.IsType((*Conn)(nil), resumableNC)

		clt, err := sshClient(resumableNC)
		require.NoError(err)
		t.Cleanup(func() { clt.Close() })

		require.Equal(sshutils.SSHVersionPrefix, string(clt.ServerVersion()))

		originalNC.Close()
		redialingSyncPoint <- struct{}{}

		_, _, err = clt.SendRequest("foo", true, nil)
		require.NoError(err)

		select {
		case redialingSyncPoint <- struct{}{}:
			require.Fail("unexpected redial")
		default:
		}

		// wait until the reconnection loop has passed the reconnection phase
		// and is waiting on the reconnection timer again
		clock.BlockUntil(1)
		clock.Advance(replacementInterval)
		redialingSyncPoint <- struct{}{}

		_, _, err = clt.SendRequest("foo", true, nil)
		require.NoError(err)
	})
}

func sshClient(nc net.Conn) (*ssh.Client, error) {
	conn, newChC, reqC, err := ssh.NewClientConn(nc, nc.RemoteAddr().String(), &ssh.ClientConfig{
		User:            "alice",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(conn, newChC, reqC), nil
}

func discardingSSHServer(t *testing.T) func(nc net.Conn) {
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		require.NoError(t, err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		require.NoError(t, err)
	}

	return func(nc net.Conn) {
		defer nc.Close()

		serverVersion := serverVersionOverrideFromConn(nc)
		if serverVersion == "" {
			serverVersion = sshutils.SSHVersionPrefix
		}
		cfg := &ssh.ServerConfig{
			NoClientAuth:  true,
			ServerVersion: serverVersion,
		}

		cfg.AddHostKey(signer)

		conn, newChC, reqC, err := ssh.NewServerConn(nc, cfg)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(reqC)
		go func() {
			for newCh := range newChC {
				newCh.Reject(ssh.UnknownChannelType, ssh.UnknownChannelType.String())
			}
		}()
		_ = conn.Wait()
	}
}

func serverVersionOverrideFromConn(nc net.Conn) string {
	for nc != nil {
		if overrider, ok := nc.(interface {
			SSHServerVersionOverride() string
		}); ok {
			if v := overrider.SSHServerVersionOverride(); v != "" {
				return v
			}
		}

		netConner, ok := nc.(interface{ NetConn() net.Conn })
		if !ok {
			break
		}
		nc = netConner.NetConn()
	}
	return ""
}
