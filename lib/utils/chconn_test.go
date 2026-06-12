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

package utils

import (
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// TestChConn validates that reads from the channel connection can be
// canceled by setting a read deadline.
func TestChConn(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	sshConnCh := make(chan sshConn)

	go startSSHServer(t, listener, sshConnCh)

	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	require.NoError(t, err)

	_, _, err = client.OpenChannel("test", []byte("hello ssh"))
	require.NoError(t, err)

	select {
	case sshConn := <-sshConnCh:
		chConn := sshutils.NewChConn(sshConn.conn, sshConn.ch)
		t.Cleanup(func() { chConn.Close() })
		doneCh := make(chan error, 1)
		go func() {
			// Nothing is sent on the channel so this will block until the
			// read is canceled by the deadline set below.
			_, err := io.ReadAll(chConn)
			doneCh <- err
		}()
		// Set the read deadline in the past and make sure that the read
		// above is canceled with a timeout error.
		chConn.SetReadDeadline(time.Unix(1, 0))
		select {
		case err := <-doneCh:
			require.True(t, os.IsTimeout(err))
		case <-time.After(time.Second):
			t.Fatal("read from channel connection wasn't canceled after 1 second")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ssh channel after 1 second")
	}
}

type sshConn struct {
	conn ssh.Conn
	ch   ssh.Channel
}

func startSSHServer(t *testing.T, listener net.Listener, sshConnCh chan<- sshConn) {
	nConn, err := listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { nConn.Close() })

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromSigner(privateKey)
	require.NoError(t, err)

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)

	conn, chans, _, err := ssh.NewServerConn(nConn, config)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	go func() {
		for newCh := range chans {
			ch, _, err := newCh.Accept()
			require.NoError(t, err)

			sshConnCh <- sshConn{
				conn: conn,
				ch:   ch,
			}
		}
	}()
}
