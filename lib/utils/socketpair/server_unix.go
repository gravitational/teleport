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

package socketpair

import (
	"net"
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

// ListenerFromFD, when used with [DialerFromFD], emulates the
// behavior of a socket server over a socket pair. The dialer side creates a new socket
// pair for every dial, sending one half to the listener listener side. This provides
// a means of easily establishing multiple connections between parent/child processes
// without needing to manage the various security and lifecycle concerns associated with
// file or network sockets.
func ListenerFromFD(fd *os.File) (net.Listener, error) {
	conn, err := unixConnFromFD(fd)
	return socketPairListener{conn}, trace.Wrap(err)
}

// DialerFromFD, when used with [ListenerFromFD], emulates the  behavior of a traditional
// socket server over a socket pair.  See [ListenerFromFD] for details.
func DialerFromFD(fd *os.File) (*Dialer, error) {
	conn, err := unixConnFromFD(fd)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Dialer{conn}, nil
}

func unixConnFromFD(fd *os.File) (*net.UnixConn, error) {
	defer fd.Close()
	fc, err := net.FileConn(fd)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, ok := fc.(*net.UnixConn)
	if !ok {
		return nil, trace.Errorf("invalid conn type %T (expected %T)", fc, (*net.UnixConn)(nil))
	}

	return conn, nil
}

type socketPairListener struct {
	c *net.UnixConn
}

// Accept implements [net.Listener].
func (l socketPairListener) Accept() (net.Conn, error) {
	// allocate space for one 32-bit fd. This will break on systems that don't
	// use 32-bit fds, but as far as I can tell the go standard library extensively
	// bakes in the assumption of 32-bit fds anyhow, so its likely this is a non-issue.
	oob := make([]byte, syscall.CmsgSpace(4))
	_, n, _, _, err := l.c.ReadMsgUnix(nil, oob)
	if n < 1 && err != nil {
		return nil, trace.Wrap(err)
	} else if n < 1 {
		return nil, trace.Errorf("received no oob data")
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:n])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var fds []int
	for _, scm := range scms {
		if scm.Header.Level != syscall.SOL_SOCKET || scm.Header.Type != syscall.SCM_RIGHTS {
			continue
		}

		f, err := syscall.ParseUnixRights(&scm)
		if err != nil {
			continue
		}

		fds = append(fds, f...)
	}

	if len(fds) < 1 {
		return nil, trace.Errorf("received no file descriptors")
	}

	for _, fd := range fds[1:] {
		// this shouldn't ever happen, but its good practice to close
		// extra fds.
		syscall.Close(fd)
	}

	f := os.NewFile(uintptr(fds[0]), "received-rights")
	nc, err := net.FileConn(f)
	f.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return nc, nil
}

// Addr implements [net.Listener].
func (l socketPairListener) Addr() net.Addr {
	return l.c.LocalAddr()
}

// Close implements net.Listener.
func (l socketPairListener) Close() error {
	return l.c.Close()
}

// Dial dials the associated listener by creating a new socketpair and passing
// one half across the main underlying socketpair.
func (d *Dialer) Dial() (net.Conn, error) {
	fd1, fd2, err := cloexecSocketpair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer syscall.Close(int(fd2))

	f1 := os.NewFile(fd1, "local-pair")
	defer f1.Close()

	nc, err := net.FileConn(f1)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, _, err = d.c.WriteMsgUnix(nil, syscall.UnixRights(int(fd2)), nil)
	if err != nil {
		nc.Close()
		return nil, trace.Wrap(err)
	}

	return nc, nil
}
