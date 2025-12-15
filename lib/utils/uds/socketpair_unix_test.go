//go:build unix

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

package uds

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// TestSocketpairFDPassing verifies the expected behavior resulting from passing combined messages and
// fds across a datagram socketpair.
func TestSocketparFDPassing(t *testing.T) {
	const maxFiles = 8

	client, server, err := NewSocketpair(SocketTypeDatagram)
	require.NoError(t, err)

	done := make(chan struct{})
	defer func() {
		close(done)
		client.Close()
		server.Close()
	}()

	// set up an echo server that echoes each message to each set of associated fds.
	go func() {
		buf := make([]byte, 1024)
		fbuf := make([]*os.File, maxFiles*2)
		for {
			n, fn, err := ReadWithFDs(server, buf, fbuf)
			if err != nil {
				select {
				case <-done:
					return
				default:
				}
				panic(fmt.Sprintf("unexpected read error: %v", err))
			}

			for _, fd := range fbuf[:fn] {
				_, err := fd.Write(buf[:n])
				if err != nil {
					panic(fmt.Sprintf("unexpected write error: %v", err))
				}
				fd.Close()
			}
		}
	}()

	var eg errgroup.Group

	for i := range maxFiles {
		f := i + 1
		eg.Go(func() error {
			msg := fmt.Sprintf("send-%d", f)
			// conns are the local halves of socket pairs that we
			// will use to read our message back from the server.
			conns := make([]*net.UnixConn, 0, f)

			// fds are the remote halves of socket pairs to be sent
			// to the server along with the associated message.
			fds := make([]*os.File, 0, f)
			for range f {
				clt, srv, err := NewSocketpair(SocketTypeStream)
				if err != nil {
					return trace.Errorf("failed to create socket pair: %v", err)
				}

				conns = append(conns, clt)
				fd, err := srv.File()
				if err != nil {
					return trace.Errorf("failed to get file from conn: %v", err)
				}
				fds = append(fds, fd)
				srv.Close()
			}

			// write message and files together so that server reads them
			// together and therefore will know what message to send back
			// over the fds.
			_, _, err := WriteWithFDs(client, []byte(msg), fds)
			if err != nil {
				return trace.Errorf("failed to write fds: %v", err)
			}

			// fds have been sent. our handles should now be closed.
			for _, fd := range fds {
				fd.Close()
			}

			// ensure that all conns have the expected message passed pack
			// across them (this is how we confirm that we are getting the
			// expected datagram behavior).
			for _, conn := range conns {
				rbytes, err := io.ReadAll(conn)
				conn.Close()
				if err != nil {
					return trace.Errorf("failed to read file: %v", err)
				}

				if rsp := string(rbytes); rsp != msg {
					return trace.Errorf("expected msg %q, got %q", msg, rsp)
				}
			}

			return nil
		})
	}

	require.NoError(t, eg.Wait())
}
