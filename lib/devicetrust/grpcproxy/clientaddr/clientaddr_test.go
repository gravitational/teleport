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

package clientaddr

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
)

func TestFromIncomingContext(t *testing.T) {
	t.Parallel()

	// proxyClientAddr is an example address of a client connecting to the Proxy
	// Service.
	proxyClientAddr := &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 51234}
	// spoofedAddr is an address a client puts in the header itself.
	const spoofedAddr = "198.51.100.1:1"

	// incoming turns the outgoing metadata of ctx into the incoming metadata of
	// a fresh context, standing in for the gRPC transport.
	incoming := func(ctx context.Context) context.Context {
		md, _ := metadata.FromOutgoingContext(ctx)
		return metadata.NewIncomingContext(t.Context(), md)
	}

	t.Run("round trip", func(t *testing.T) {
		for _, test := range []struct {
			name string
			addr *net.TCPAddr
		}{
			{
				name: "IPv4",
				addr: &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 51234},
			},
			{
				name: "IPv6",
				addr: &net.TCPAddr{IP: net.ParseIP("2001:db8::7"), Port: 51234},
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				got, err := FromIncomingContext(incoming(WithOutgoingContext(t.Context(), test.addr)))
				require.NoError(t, err)
				assert.Equal(t, test.addr.String(), got.String())
			})
		}
	})

	t.Run("drops the zone", func(t *testing.T) {
		zoned := &net.TCPAddr{IP: net.ParseIP("fe80::7"), Port: 51234, Zone: "eth0"}
		got, err := FromIncomingContext(incoming(WithOutgoingContext(t.Context(), zoned)))
		require.NoError(t, err)
		assert.Empty(t, got.Zone)
		assert.Equal(t, "[fe80::7]:51234", got.String())
	})

	t.Run("replaces a client-supplied value", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(t.Context(), Header, spoofedAddr)
		got, err := FromIncomingContext(incoming(WithOutgoingContext(ctx, proxyClientAddr)))
		require.NoError(t, err)
		assert.Equal(t, proxyClientAddr.String(), got.String())
	})

	// The proxy sets the header from the connection and overwrites whatever the
	// client sent, so none of these failures can be the client's doing. They
	// must reach the client as an unclassified server error, not with a kind
	// that blames the client's request, the way BadParameter or NotFound would.
	tests := []struct {
		name    string
		md      metadata.MD
		wantErr string
	}{
		{
			name:    "missing",
			md:      metadata.MD{},
			wantErr: "did not forward the client address",
		},
		{
			name:    "repeated",
			md:      metadata.Pairs(Header, proxyClientAddr.String(), Header, spoofedAddr),
			wantErr: "repeated",
		},
		{
			name:    "malformed",
			md:      metadata.Pairs(Header, "not an address"),
			wantErr: "parse client address",
		},
		{
			name:    "no port",
			md:      metadata.Pairs(Header, "203.0.113.7"),
			wantErr: "parse client address",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := FromIncomingContext(metadata.NewIncomingContext(t.Context(), test.md))
			require.Error(t, err)
			assert.ErrorContains(t, err, test.wantErr)
			assert.Equal(t, codes.Unknown, status.Code(trail.ToGRPC(err)))
		})
	}
}
