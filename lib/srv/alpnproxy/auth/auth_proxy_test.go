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

package alpnproxyauth

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/observability/tracing"
)

func TestDialLocalAuthServerNoServers(t *testing.T) {
	s := NewAuthProxyDialerService(nil /* reverseTunnelServer */, "clustername", nil /* authServers */, nil, tracing.NoopTracer("test"))
	_, err := s.dialLocalAuthServer(context.Background(), nil, nil)
	require.Error(t, err, "dialLocalAuthServer expected to fail")
	require.Equal(t, "empty auth servers list", err.Error())
}

func TestDialLocalAuthServerNoAvailableServers(t *testing.T) {
	// The 203.0.113.0/24 range is part of block TEST-NET-3 as defined in RFC-5735 (https://www.rfc-editor.org/rfc/rfc5735).
	// IPs in this range do not appear on the public internet.
	s := NewAuthProxyDialerService(nil /* reverseTunnelServer */, "clustername", []string{"203.0.113.1:3025"}, nil, tracing.NoopTracer("test"))
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	t.Cleanup(cancel)
	_, err := s.dialLocalAuthServer(ctx, nil, nil)
	require.Error(t, err, "dialLocalAuthServer expected to fail")
	var netErr *net.OpError
	require.ErrorAs(t, err, &netErr)
	require.Equal(t, "dial", netErr.Op)
	require.Equal(t, "203.0.113.1:3025", netErr.Addr.String())
}

func TestDialLocalAuthServerAvailableServers(t *testing.T) {
	socket, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, socket.Close()) })

	authServers := make([]string, 1, 11)
	authServers[0] = socket.Addr().String()
	// multiple invalid servers to minimize chance that we select good one first try
	for i := range 10 {
		// The 203.0.113.0/24 range is part of block TEST-NET-3 as defined in RFC-5735 (https://www.rfc-editor.org/rfc/rfc5735).
		// IPs in this range do not appear on the public internet.
		authServers = append(authServers, fmt.Sprintf("203.0.113.%d:3025", i+1))
	}
	s := NewAuthProxyDialerService(nil /* reverseTunnelServer */, "clustername", authServers, nil, tracing.NoopTracer("test"))
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		t.Cleanup(cancel)
		conn, err := s.dialLocalAuthServer(ctx, nil, nil)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 5*time.Second, 10*time.Millisecond)
}
