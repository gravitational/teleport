// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package devicetrust

import (
	"context"
	"net"
	"strconv"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/metadata"
)

// clientSrcAddrKey is the gRPC metadata key carrying the source address of the
// device that called a public Device Trust RPC.
//
// The public RPCs are unauthenticated from the end user's perspective, so a
// device reaches the Auth Service through the Proxy Service and Auth sees the
// Proxy as its peer. Without the forwarded address Auth has no way to tell
// where the device connected from.
const clientSrcAddrKey = "devicetrust-client-src-addr"

// WithForwardedClientSrcAddr returns a context that carries addr to the Auth
// Service on outgoing public Device Trust RPCs.
//
// Only the Proxy Service may set this, see
// [ForwardedClientSrcAddrFromContext].
func WithForwardedClientSrcAddr(ctx context.Context, addr net.Addr) context.Context {
	return metadata.AppendToOutgoingContext(ctx, clientSrcAddrKey, addr.String())
}

// ForwardedClientSrcAddrFromContext returns the device source address that the
// Proxy Service forwarded with an incoming public Device Trust RPC.
//
// Any client can attach metadata to a request, so a caller must establish that
// the peer holds the Proxy builtin role before it trusts the result.
func ForwardedClientSrcAddrFromContext(ctx context.Context) (*net.TCPAddr, error) {
	vals := metadata.ValueFromIncomingContext(ctx, clientSrcAddrKey)
	switch len(vals) {
	case 1:
	case 0:
		return nil, trace.NotFound("client source address not forwarded")
	default:
		// A single Proxy hop sets the key exactly once. Picking one of several
		// values would let whoever supplied the extra one choose the address
		// that gets enforced.
		return nil, trace.BadParameter("client source address forwarded more than once")
	}

	host, port, err := net.SplitHostPort(vals[0])
	if err != nil {
		return nil, trace.Wrap(err, "parse forwarded client source address")
	}

	// Reject anything that would need resolving: the address has to be the one
	// the Proxy observed, and IP pinning compares numeric addresses.
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, trace.BadParameter("forwarded client source address %q is not an IP address", host)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, trace.BadParameter("forwarded client source port %q is not a number", port)
	}

	return &net.TCPAddr{IP: ip, Port: portNum}, nil
}
