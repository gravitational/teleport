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

package tcpip

import (
	"crypto/rand"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/socketpair"
)

// TestForwarder verifies basic forwarding operations using a tcp echo server.
func TestForwarder(t *testing.T) {
	t.Parallel()

	left, right, err := socketpair.NewFDs()
	require.NoError(t, err)

	socketListener, err := socketpair.ListenerFromFD(left)
	require.NoError(t, err)

	socketDialer, err := socketpair.DialerFromFD(right)
	require.NoError(t, err)

	forwarder := NewForwarder(socketListener)

	dialer := NewDialer(socketDialer)

	fwdResult := make(chan error, 1)

	go func() {
		fwdResult <- forwarder.Run()
	}()
	defer forwarder.Close()

	// set up a simple tcp echo server to test forwarding against
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer tcpListener.Close()

	go func() {
		for {
			conn, err := tcpListener.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()
				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf[:])
					if err != nil {
						if err != io.EOF {
							panic(err)
						}
						return
					}

					if _, err := conn.Write(buf[:n]); err != nil {
						panic(err)
					}
				}
			}()
		}
	}()

	conn, err := dialer.Dial(tcpListener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	msg := []byte("hello there!")
	_, err = conn.Write(msg)
	require.NoError(t, err)

	buf := make([]byte, len(msg))

	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, msg, buf)

	// verify that closing the dialer kills the forwarder
	dialer.Close()
	select {
	case err := <-fwdResult:
		require.NoError(t, err)
	case <-time.After(time.Second * 30):
		require.FailNow(t, "timeout waiting for forwarder to exit")
	}
}

func TestLengthPrefixedMessage(t *testing.T) {
	t.Parallel()

	msgs := [][]byte{
		nil,
		[]byte{0},
		[]byte{0xff},
		[]byte("Hello there!"),
	}

	largeMsg := make([]byte, maxLengthPrefixedMessageSize)
	_, err := rand.Read(largeMsg)
	require.NoError(t, err)

	msgs = append(msgs, largeMsg)

	for _, msg := range msgs {
		checkUnary(t, msg)
	}

	checkStreaming(t, msgs)
}

// checkMsgSend checks that a single length-prefixed message is correctly preserved across
// encoding/decoding.
func checkUnary(t *testing.T, original []byte) {
	left, right := net.Pipe()
	werr := make(chan error, 1)
	go func() {
		werr <- WriteLengthPrefixedMessage(left, original)
	}()

	output, err := ReadLengthPrefixedMessage(right)
	require.NoError(t, err)

	require.Equal(t, original, output)

	require.NoError(t, <-werr)
}

// checkStreaming verifies expected behavior of streaming length-prefixed messages unidirectionally.
func checkStreaming(t *testing.T, msgs [][]byte) {
	left, right := net.Pipe()

	// start writer that will send all messsages and then close the pipe
	werr := make(chan error, 1)
	go func() {
		defer left.Close()
		for _, msg := range msgs {
			if err := WriteLengthPrefixedMessage(left, msg); err != nil {
				werr <- err
				return
			}
		}
	}()

	// start reader that will pull all messages and then exit gracefully on EOF
	// (note: EOF is only returned if the closure happens between messages, a
	// closure mid-message results in UnexpectedEOF).
	rerr := make(chan error, 1)
	rmsgs := make(chan []byte, len(msgs))
	rdone := make(chan struct{})
	go func() {
		defer close(rdone)
		for {
			msg, err := ReadLengthPrefixedMessage(right)
			if err != nil {
				if err != io.EOF {
					rerr <- err
				}
				return
			}
			rmsgs <- msg
		}
	}()

	timeout := time.After(30 * time.Second)

	// verify that expected messages are correctly propagated from writer to reader.
	for _, expect := range msgs {
		select {
		case msg := <-rmsgs:
			require.Equal(t, expect, msg)
		case err := <-werr:
			require.FailNowf(t, "writer failed", "error: %v", err)
		case err := <-rerr:
			require.FailNowf(t, "reader failed", "error: %v", err)
		case <-timeout:
			require.FailNow(t, "timeout waiting for messages")
		}
	}

	// verify that EOF correctly propagated, causing reader to exit gracefully
	select {
	case <-rdone:
		// success
	case err := <-werr:
		require.FailNowf(t, "writer failed", "error: %v", err)
	case err := <-rerr:
		require.FailNowf(t, "reader failed", "error: %v", err)
	case <-timeout:
		require.FailNow(t, "timeout waiting for messages")
	}
}
