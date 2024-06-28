//go:build unix

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
)

// ListenConfig is an alias of [net.ListenConfig] with methods for creating Unix
// domain socket listeners without type assertions and possibly supporting
// oversized paths (depending on the platform).
type ListenConfig net.ListenConfig

// assert the signature of platform-specific methods
var _ interface {
	ListenUnix(ctx context.Context, network, path string) (*net.UnixListener, error)
	ListenUnixgram(ctx context.Context, network, path string) (*net.UnixConn, error)
} = (*ListenConfig)(nil)

// ListenUnix is like [net.ListenUnix] but with a context and potentially
// supporting oversized paths.
func ListenUnix(ctx context.Context, network, path string) (*net.UnixListener, error) {
	return new(ListenConfig).ListenUnix(ctx, network, path)
}

// ListenUnixgram is like [net.ListenUnixgram] but with a context and
// potentially supporting oversized paths.
func ListenUnixgram(ctx context.Context, network, path string) (*net.UnixConn, error) {
	return new(ListenConfig).ListenUnixgram(ctx, network, path)
}

// Dialer is an alias of [net.Dialer] with a method for dialing Unix domain
// sockets without type assertions and possibly supporting oversized paths
// (depending on the platform).
type Dialer net.Dialer

var _ interface {
	DialUnix(ctx context.Context, network, path string) (*net.UnixConn, error)
} = (*Dialer)(nil)

// DialUnix is like [net.DialUnix] but with a context and potentially supporting
// oversized paths.
func DialUnix(ctx context.Context, network, path string) (*net.UnixConn, error) {
	return new(Dialer).DialUnix(ctx, network, path)
}
