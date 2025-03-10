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
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/lib/multiplexer"
)

func TestAlreadyWritten(t *testing.T) {
	require := require.New(t)

	lc := new(logConn)
	c := &sshVersionSkipConn{
		Conn: lc,

		alreadyWritten: "aa",
	}

	n, err := c.Write([]byte("a"))
	require.NoError(err)
	require.Equal(1, n)
	require.Equal("a", c.alreadyWritten)

	n, err = c.Write([]byte("b"))
	require.Error(err)
	require.ErrorAs(err, new(*trace.BadParameterError))
	require.Equal(0, n)

	n, err = c.Write([]byte("ab"))
	require.NoError(err)
	require.Equal(2, n)
	require.Empty(c.alreadyWritten)
	require.Equal([]byte("b"), lc.log.Bytes())
}

type logConn struct {
	net.Conn
	log bytes.Buffer
}

func (c *logConn) Write(p []byte) (int, error) {
	return c.log.Write(p)
}

func TestFixedHeader(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(err)
	t.Cleanup(func() { listener.Close() })

	const defaultSSHVersionIdentifier = "SSH-2.0-Go"
	mux, err := multiplexer.New(multiplexer.Config{
		Listener:  listener,
		PreDetect: PreDetectFixedSSHVersion(defaultSSHVersionIdentifier),
	})
	require.NoError(err)
	t.Cleanup(func() { mux.Close() })
	go mux.Serve()

	go serveOneSSH(t, mux.SSH())

	netConn, err := net.DialTimeout(listener.Addr().Network(), listener.Addr().String(), 5*time.Second)
	require.NoError(err)
	t.Cleanup(func() { netConn.Close() })

	// the SSH transport layer protocol rfc (5423) states that SSH servers must
	// send a version string immediately after the connection is established, so
	// we expect (a specific) version string without sending anything
	buf := make([]byte, len(defaultSSHVersionIdentifier))
	_, err = io.ReadFull(netConn, buf)
	require.NoError(err)
	require.Equal(defaultSSHVersionIdentifier, string(buf))

	// the SSH server hasn't even been touched yet, so we can connect to it from
	// a separate connection (we have to, in fact, or serveOneSSH will fail
	// the test)

	sshClient, err := ssh.Dial(listener.Addr().Network(), listener.Addr().String(), &ssh.ClientConfig{
		User:            "bob",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	require.NoError(err)
	t.Cleanup(func() { sshClient.Close() })

	const payload = "this is a bit useless since we already went through a full handshake"
	ok, echoReply, err := sshClient.Conn.SendRequest("echo", true, []byte(payload))
	require.NoError(err)
	require.True(ok)
	require.Equal(payload, string(echoReply))
}

func serveOneSSH(t *testing.T, listener net.Listener) {
	assert := assert.New(t)

	nc, err := listener.Accept()
	if !assert.NoError(err) {
		return
	}
	t.Cleanup(func() { _ = nc.Close() })

	_, privKey, err := cryptopatch.GenerateEd25519Key(nil)
	if !assert.NoError(err) {
		return
	}

	hostKey, err := ssh.NewSignerFromKey(privKey)
	assert.NoError(err)

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(hostKey)

	conn, newChC, reqC, err := ssh.NewServerConn(nc, config)
	if !assert.NoError(err) {
		return
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		for newCh := range newChC {
			err := newCh.Reject(ssh.UnknownChannelType, ssh.UnknownChannelType.String())
			assert.NoError(err)
		}
	}()

	go func() {
		for newReq := range reqC {
			err := newReq.Reply(newReq.Type == "echo", newReq.Payload)
			assert.NoError(err)
		}
	}()

	_ = conn.Wait()
}
