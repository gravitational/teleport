//go:build unix && !darwin && !linux

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

	"github.com/gravitational/trace"
)

// ListenUnix is like [net.ListenUnix] but with a context (or like
// [net.ListenConfig.Listen] without a type assertion). The network must be
// "unix" or "unixpacket". On this platform, the path must fit in a sockaddr_un
// struct.
func (lc *ListenConfig) ListenUnix(ctx context.Context, network, path string) (*net.UnixListener, error) {
	switch network {
	case "unix", "unixpacket":
	default:
		return nil, trace.BadParameter("invalid network %q, expected \"unix\" or \"unixpacket\"", network)
	}

	l, err := (*net.ListenConfig)(lc).Listen(ctx, network, path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.(*net.UnixListener), nil
}

// ListenUnixgram is like [net.ListenUnixgram] but with a context (or like
// [net.ListenConfig.ListenPacket] without a type assertion). The network must
// be "unixgram". On this platform, the path must fit in a sockaddr_un struct.
func (lc *ListenConfig) ListenUnixgram(ctx context.Context, network, path string) (*net.UnixConn, error) {
	switch network {
	case "unixgram":
	default:
		return nil, trace.BadParameter("invalid network %q, expected \"unixgram\"", network)
	}

	l, err := (*net.ListenConfig)(lc).ListenPacket(ctx, network, path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.(*net.UnixConn), nil
}

// DialUnix is like [net.DialUnix] but with a context (or like
// [net.Dialer.DialContext] without a type assertion). The network must be
// "unix", "unixgram" or "unixpacket". On this platform, the path must fit in a
// sockaddr_un struct.
func (d *Dialer) DialUnix(ctx context.Context, network, path string) (*net.UnixConn, error) {
	switch network {
	case "unix", "unixgram", "unixpacket":
	default:
		return nil, trace.BadParameter("invalid network %q, expected \"unix\", \"unixgram\" or \"unixpacket\"", network)
	}

	l, err := (*net.Dialer)(d).DialContext(ctx, network, path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return l.(*net.UnixConn), nil
}
