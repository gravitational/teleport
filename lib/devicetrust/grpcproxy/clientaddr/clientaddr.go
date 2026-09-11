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

// Package clientaddr carries the source address of a Device Trust client from
// the Proxy Service to the Auth Service in gRPC metadata.
package clientaddr

import (
	"context"
	"net"
	"net/netip"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/metadata"
)

// Header is the gRPC metadata key the Proxy Service uses to forward
// the address of the client to the Auth Service. The Auth Service sees
// the Proxy Service as the peer of every public Device Trust RPC, so a check
// that needs the address the device connected from (IP pinning for example)
// needs it forwarded.
//
// The Auth Service trusts the header only from callers that authenticate as a
// Proxy, and the Proxy Service always overwrites whatever value the client
// sent.
//
// See RFD 32e for more details.
const Header = "devicetrust-client-src-addr"

// WithOutgoingContext sets addr as the forwarded client address in the
// outgoing metadata of ctx, replacing any value already there.
//
// Only TCP addresses are carried, since the readers need the IP and the port.
func WithOutgoingContext(ctx context.Context, addr *net.TCPAddr) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	md.Set(Header, addr.String())
	return metadata.NewOutgoingContext(ctx, md)
}

// FromIncomingContext reads the client address forwarded by the Proxy
// Service from the incoming metadata of ctx.
//
// Callers must check that the request comes from a Proxy before trusting the
// result.
func FromIncomingContext(ctx context.Context) (*net.TCPAddr, error) {
	// Every error here describes a Proxy that sent nothing usable, which a client
	// has no way to bring about. None of them carry a [trace] kind, so they reach
	// the client as an unclassified server-side failure instead of something it
	// might act on.
	md, _ := metadata.FromIncomingContext(ctx)
	switch vals := md.Get(Header); len(vals) {
	case 0:
		return nil, trace.Errorf("the Proxy Service did not forward the client address")
	case 1:
		addrPort, err := netip.ParseAddrPort(vals[0])
		if err != nil {
			return nil, trace.Wrap(err, "parse client address header")
		}
		// Remove the zone to improve the error message in case of a zoned address.
		//
		// A zone only arrives from a client that planted one in a PROXY protocol
		// header (lib/multiplexer.PP2TeleportSubtypeOriginalAddr).
		// authz.CheckIPPinning parses the client address with net.ParseIP, which
		// rejects a zone. Without removing the zone, the request fails with an
		// unexplained denial rather than an IP mismatch.
		// Stripping the zone is not a security measure. A zoned address cannot be
		// used to get past IP pinning.
		addr := addrPort.Addr().WithZone("")
		return net.TCPAddrFromAddrPort(netip.AddrPortFrom(addr, addrPort.Port())), nil
	default:
		// The proxy sets a single value. More than one means a value from the
		// client got through.
		return nil, trace.Errorf("client address header repeated")
	}
}
