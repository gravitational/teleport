//go:build unix

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

package networking

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/sshutils/reexec"
	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestReadResponseSuccess(t *testing.T) {
	conn, remoteConn, err := uds.NewSocketpair(uds.SocketTypeStream)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
		_ = remoteConn.Close()
	})

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	writeErr := make(chan error, 1)
	go func() {
		defer remoteConn.Close()
		_, _, err := uds.WriteWithFDs(remoteConn, []byte("ok"), []*os.File{writer})
		writeErr <- err
	}()

	file, err := readResponse(conn)
	require.NoError(t, err)
	require.NotNil(t, file)
	t.Cleanup(func() { _ = file.Close() })

	_, err = file.Write([]byte("ping"))
	require.NoError(t, err)

	buf := make([]byte, 4)
	_, err = io.ReadFull(reader, buf)
	require.NoError(t, err)
	require.Equal(t, "ping", string(buf))

	require.NoError(t, <-writeErr)
}

func TestReadResponseLargeError(t *testing.T) {
	conn, remoteConn, err := uds.NewSocketpair(uds.SocketTypeStream)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
		_ = remoteConn.Close()
	})

	errMsg := strings.Repeat("x", RequestBufferSize+16)
	go func() {
		_, _ = remoteConn.Write([]byte(errMsg))
		_ = remoteConn.Close()
	}()

	file, err := readResponse(conn)
	require.Error(t, err)
	require.Nil(t, file)
	require.Contains(t, err.Error(), errMsg)
}

func TestSendRequestSuccess(t *testing.T) {
	ctx := context.Background()
	proc, remoteConn := newTestProcess(t)

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	req := Request{
		Operation: NetworkingOperationDial,
		Network:   "tcp",
		Address:   "127.0.0.1:0",
	}

	errCh := make(chan error, 1)
	go func() {
		defer remoteConn.Close()
		buf := make([]byte, RequestBufferSize)
		fbuf := make([]*os.File, 1)
		n, fn, err := uds.ReadWithFDs(remoteConn, buf, fbuf)
		if err != nil {
			errCh <- err
			return
		}
		if fn != 1 {
			errCh <- trace.Errorf("expected 1 fd, got %d", fn)
			return
		}

		var got Request
		if err := json.Unmarshal(buf[:n], &got); err != nil {
			errCh <- err
			return
		}
		if got.Operation != req.Operation || got.Network != req.Network || got.Address != req.Address || got.X11Request != req.X11Request {
			errCh <- trace.Errorf("unexpected request: %+v", got)
			return
		}

		requestConn, err := uds.FromFile(fbuf[0])
		if err != nil {
			errCh <- err
			return
		}
		defer func() {
			_ = requestConn.Close()
			_ = fbuf[0].Close()
		}()

		if _, _, err := uds.WriteWithFDs(requestConn, []byte("ok"), []*os.File{writer}); err != nil {
			errCh <- err
			return
		}

		errCh <- nil
	}()

	file, err := proc.sendRequest(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, file)
	t.Cleanup(func() { _ = file.Close() })

	_, err = file.Write([]byte("pong"))
	require.NoError(t, err)

	buf := make([]byte, 4)
	_, err = io.ReadFull(reader, buf)
	require.NoError(t, err)
	require.Equal(t, "pong", string(buf))

	require.NoError(t, <-errCh)
}

func TestSendRequestTimeout(t *testing.T) {
	proc, remoteConn := newTestProcess(t)
	t.Cleanup(func() { _ = remoteConn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := proc.sendRequest(ctx, Request{
		Operation: NetworkingOperationDial,
		Network:   "tcp",
		Address:   "127.0.0.1:0",
	})
	require.Error(t, err)
	require.Less(t, time.Since(start), time.Second)
}

func newTestProcess(t *testing.T) (*Process, *net.UnixConn) {
	t.Helper()

	cmd, err := reexec.NewCommand(&reexec.Config{
		ReexecCommand: "test",
		LogWriter:     io.Discard,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = cmd.Close() })

	remoteConn, localConn, err := uds.NewSocketpair(uds.SocketTypeDatagram)
	require.NoError(t, err)
	t.Cleanup(func() { _ = localConn.Close() })

	return &Process{
		cmd:  cmd,
		conn: localConn,
	}, remoteConn
}
