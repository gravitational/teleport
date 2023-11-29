/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tcpip

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestForwarderKeepalive verifies that keepalive messages are understood by
// the forwarder and they they result in an updated (more recent) 'last active'
// time being set for the forwarder.
func TestForwarderKeepalive(t *testing.T) {
	const iotimeout = time.Second * 10

	t.Parallel()

	dir := t.TempDir()

	addr, pfd, err := SetupListenerFD(dir)
	require.NoError(t, err)

	cfd := os.NewFile(pfd.Fd(), addr)

	forwarder, err := NewForwarder(cfd, time.Minute)
	require.NoError(t, err)
	go func() {
		if err := forwarder.Run(); err != nil {
			logrus.Warnf("Forwarder exited with error: %v", err)
		}
	}()
	defer forwarder.Close()

	require.NoError(t, keepaliveSocket(addr, iotimeout))

	for i := 0; i < 5; i++ {
		pre, _ := forwarder.getLastActive()
		require.NoError(t, keepaliveSocket(addr, iotimeout))
		post, _ := forwarder.getLastActive()
		require.True(t, post.After(pre))
	}
}

// TestForwarderBasics verifies basic forwarding operations using a tcp echo server.
func TestForwarderBasics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	addr, pfd, err := SetupListenerFD(dir)
	require.NoError(t, err)

	cfd := os.NewFile(pfd.Fd(), addr)

	forwarder, err := NewForwarder(cfd, time.Minute)
	require.NoError(t, err)
	go func() {
		if err := forwarder.Run(); err != nil {
			logrus.Warnf("Forwarder exited with error: %v", err)
		}
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
						return
					}
				}
			}()
		}
	}()

	conn, err := DialThroughForwarder(addr, tcpListener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	msg := []byte("hello there!")
	_, err = conn.Write(msg)
	require.NoError(t, err)

	buf := make([]byte, len(msg))

	_, err = io.ReadFull(conn, buf)
	require.NoError(t, err)
	require.Equal(t, msg, buf)

	// verify that cleanup logic views forwarder as "alive" and does not try
	// to clean up the socket.
	var state map[string]struct{}
	for i := 0; i < 10; i++ {
		var err error
		state, err = cleanSocks(dir, state, time.Second)
		require.NoError(t, err)

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	}

	forwarder.Close()

	// explicit closure should result in cleanup
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 0)
}

// TestSocketOps verifies the expected behavior of the socket creation and cleanup helpers.
func TestSocketOps(t *testing.T) {
	const (
		socketCount = 8
		iotimeout   = time.Second
	)

	t.Parallel()

	dir := t.TempDir()

	socks := make([]net.Listener, 0, socketCount)

	var wg sync.WaitGroup
	for i := 0; i < socketCount; i++ {
		addr, pfd, err := SetupListenerFD(dir)
		require.NoError(t, err)

		cfd := os.NewFile(pfd.Fd(), addr)

		listener, err := ListenerFromFD(cfd)
		require.NoError(t, err)

		// don't unlink on close so that we can test cleanup
		listener.(*net.UnixListener).SetUnlinkOnClose(false)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				conn, err := listener.Accept()
				if err != nil {
					if !utils.IsOKNetworkError(err) {
						logrus.Warnf("Socket %q closing due to error: %v", addr, err)
					}
					return
				}
				go func() {
					defer conn.Close()
					msg, err := ReadLengthPrefixedMessage(conn)
					if err != nil {
						logrus.Warnf("Failed to read expected ping msg: %v", err)
						return
					}

					if !bytes.Equal(msg, pingMsg) {
						logrus.Warnf("Unexpected ping msg: %v", err)
						return
					}

					if err := WriteLengthPrefixedMessage(conn, pongMsg); err != nil {
						logrus.Warnf("Failed to send pong msg: %v", err)
						return
					}
				}()
			}
		}()

		require.NoError(t, pingSocket(addr, 9*time.Second))

		socks = append(socks, listener)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, socketCount)

	var state map[string]struct{}
	for i := 0; i < 10; i++ {
		var err error
		state, err = cleanSocks(dir, state, iotimeout)
		require.NoError(t, err)
	}

	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, socketCount)

	socksToClose := socks[:socketCount/2]
	socks = socks[socketCount/2:]

	// close half the sockets
	for _, sock := range socksToClose {
		sock.Close()
	}

	time.Sleep(time.Second)

	// first pass should have no effect, only marking unhealthy
	// sockets for future cleanup.
	state, err = cleanSocks(dir, state, iotimeout)
	require.NoError(t, err)
	require.Len(t, state, socketCount/2)

	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, socketCount)

	// second pass should result in cleanup being triggered
	// for the closed sockets.
	state, err = cleanSocks(dir, state, iotimeout)
	require.NoError(t, err)
	require.Empty(t, state)

	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, socketCount/2)

	// subsequent calls should not close further sockets
	for i := 0; i < 10; i++ {
		state, err = cleanSocks(dir, state, iotimeout)
		require.NoError(t, err)
		require.Empty(t, state)
	}

	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, socketCount/2)

	// close second half of the sockets
	for _, sock := range socks {
		sock.Close()
	}

	// clean up remaining sockets
	for i := 0; i < 2; i++ {
		state, err = cleanSocks(dir, state, iotimeout)
		require.NoError(t, err)
	}

	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)

	// sanity check: background goroutines should all have exited
	// (or be in the process of exiting) by this point.
	wg.Wait()
}

func TestLengthPrefixedMessageBasics(t *testing.T) {
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

// checkStreaming verifies expected behavior of streaming length-prefixed messages uniderectionally.
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
