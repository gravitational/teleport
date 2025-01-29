//go:build darwin

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

package uds

import (
	"context"
	"net"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// sunPathLen is the size of [unix.RawSockaddrUnix.Path]; spelled out as a
// number so that we don't have to import unsafe just for
// unsafe.Sizeof(unix.RawSockaddrUnix{}.Path).
const sunPathLen = 104

// static check that [sunPathLen] is correct
var _ [sunPathLen]int8 = unix.RawSockaddrUnix{}.Path

// ListenUnix is like [net.ListenUnix] but with a context (or like
// [net.ListenConfig.Listen] without a type assertion). The network must be
// "unix" or "unixpacket". On this platform (darwin), only the last component of
// the path must fit in sockaddr_un (104 characters), as oversized paths are
// handled by changing directory before binding the socket.
func (lc *ListenConfig) ListenUnix(ctx context.Context, network, path string) (*net.UnixListener, error) {
	switch network {
	case "unix", "unixpacket":
	default:
		return nil, trace.BadParameter("invalid network %q, expected \"unix\" or \"unixpacket\"", network)
	}

	if strings.IndexByte(path, '\x00') != -1 {
		return nil, trace.BadParameter("path must not contain NUL")
	}

	if len(path) > sunPathLen {
		path = filepath.Clean(path)
	}

	if len(path) <= sunPathLen {
		l, err := (*net.ListenConfig)(lc).Listen(ctx, network, path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return l.(*net.UnixListener), nil
	}

	dir, file := filepath.Split(path)
	if len(file) > sunPathLen {
		return nil, trace.BadParameter("final path component is too long")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := unix.PthreadChdir(dir); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}
	defer func() {
		if err := unix.PthreadFchdir(-1); err != nil {
			panic(err)
		}
	}()

	l, err := (*net.ListenConfig)(lc).Listen(ctx, network, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.(*net.UnixListener), err
}

// ListenUnixgram is like [net.ListenUnixgram] but with a context (or like
// [net.ListenConfig.ListenPacket] without a type assertion). The network must
// be "unixgram". On this platform (darwin), only the last component of the path
// must fit in sockaddr_un (104 characters), as oversized paths are handled by
// changing directory before binding the socket.
func (lc *ListenConfig) ListenUnixgram(ctx context.Context, network, path string) (*net.UnixConn, error) {
	switch network {
	case "unixgram":
	default:
		return nil, trace.BadParameter("invalid network %q, expected \"unixgram\"", network)
	}

	if strings.IndexByte(path, '\x00') != -1 {
		return nil, trace.BadParameter("path must not contain NUL")
	}

	if len(path) > sunPathLen {
		path = filepath.Clean(path)
	}

	if len(path) <= sunPathLen {
		l, err := (*net.ListenConfig)(lc).ListenPacket(ctx, network, path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return l.(*net.UnixConn), nil
	}

	dir, file := filepath.Split(path)
	if len(file) > sunPathLen {
		return nil, trace.BadParameter("final path component is too long")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := unix.PthreadChdir(dir); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}
	defer func() {
		if err := unix.PthreadFchdir(-1); err != nil {
			panic(err)
		}
	}()

	l, err := (*net.ListenConfig)(lc).ListenPacket(ctx, network, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.(*net.UnixConn), err
}

// DialUnix is like [net.DialUnix] but with a context (or like
// [net.Dialer.DialContext] without a type assertion). The network must be
// "unix", "unixgram" or "unixpacket". On this platform (darwin), only the last
// component of the path must fit in sockaddr_un (104 characters), as oversized
// paths are handled by changing directory before binding the socket.
func (d *Dialer) DialUnix(ctx context.Context, network, path string) (*net.UnixConn, error) {
	switch network {
	case "unix", "unixgram", "unixpacket":
	default:
		return nil, trace.BadParameter("invalid network %q, expected \"unix\", \"unixgram\" or \"unixpacket\"", network)
	}

	if strings.IndexByte(path, '\x00') != -1 {
		return nil, trace.BadParameter("path must not contain NUL")
	}

	if len(path) > sunPathLen {
		path = filepath.Clean(path)
	}

	if len(path) <= sunPathLen {
		c, err := (*net.Dialer)(d).DialContext(ctx, network, path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return c.(*net.UnixConn), nil
	}

	dir, file := filepath.Split(path)
	if len(file) > sunPathLen {
		return nil, trace.BadParameter("final path component is too long")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := unix.PthreadChdir(dir); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}
	defer func() {
		if err := unix.PthreadFchdir(-1); err != nil {
			panic(err)
		}
	}()

	c, err := (*net.Dialer)(d).DialContext(ctx, network, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.(*net.UnixConn), err
}
