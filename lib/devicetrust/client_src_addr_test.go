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

package devicetrust_test

import (
	"context"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/lib/devicetrust"
)

func TestForwardedClientSrcAddr(t *testing.T) {
	t.Parallel()

	t.Run("round trips the address the proxy observed", func(t *testing.T) {
		want := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4242}

		got, err := devicetrust.ForwardedClientSrcAddrFromContext(
			asIncoming(t, devicetrust.WithForwardedClientSrcAddr(t.Context(), want)))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("round trips an IPv6 address", func(t *testing.T) {
		want := &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 4242}

		got, err := devicetrust.ForwardedClientSrcAddrFromContext(
			asIncoming(t, devicetrust.WithForwardedClientSrcAddr(t.Context(), want)))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns NotFound when nothing was forwarded", func(t *testing.T) {
		_, err := devicetrust.ForwardedClientSrcAddrFromContext(t.Context())
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
	})

	// A second value would otherwise let whoever supplied it pick the address
	// that gets enforced.
	t.Run("rejects a second forwarded address", func(t *testing.T) {
		ctx := devicetrust.WithForwardedClientSrcAddr(t.Context(),
			&net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4242})
		ctx = devicetrust.WithForwardedClientSrcAddr(ctx,
			&net.TCPAddr{IP: net.ParseIP("192.0.2.11"), Port: 4242})

		_, err := devicetrust.ForwardedClientSrcAddrFromContext(asIncoming(t, ctx))
		assert.ErrorAs(t, err, new(*trace.BadParameterError))
		assert.ErrorContains(t, err, "more than once")
	})

	for _, test := range []struct {
		name, addr, wantErr string
	}{
		{
			name:    "no port",
			addr:    "192.0.2.10",
			wantErr: "parse forwarded client source address",
		},
		{
			name:    "hostname instead of an IP",
			addr:    "device.example.com:4242",
			wantErr: "is not an IP address",
		},
		{
			name:    "named port",
			addr:    "192.0.2.10:https",
			wantErr: "is not a number",
		},
		{
			name:    "empty",
			addr:    "",
			wantErr: "parse forwarded client source address",
		},
	} {
		t.Run("rejects "+test.name, func(t *testing.T) {
			// Spelled out rather than taken from the package, so that a change
			// to the wire key shows up as a failure here.
			ctx := metadata.NewIncomingContext(t.Context(),
				metadata.Pairs("devicetrust-client-src-addr", test.addr))

			_, err := devicetrust.ForwardedClientSrcAddrFromContext(ctx)
			assert.Error(t, err)
			assert.ErrorContains(t, err, test.wantErr)
		})
	}
}

// asIncoming turns the outgoing metadata that [devicetrust.WithForwardedClientSrcAddr]
// sets into incoming metadata, standing in for the gRPC round trip.
func asIncoming(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok, "no outgoing metadata in context")
	return metadata.NewIncomingContext(ctx, md)
}
