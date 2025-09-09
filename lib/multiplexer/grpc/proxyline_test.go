// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package grpc

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/lib/multiplexer"
)

func TestPPV2ServerCredentials(t *testing.T) {
	dispatchC := make(chan net.Conn, 1)

	creds := PPV2ServerCredentials{TransportCredentials: dispatchCredentials{dispatchC: dispatchC}}

	srv := grpc.NewServer(grpc.Creds(creds))
	defer srv.Stop()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()
	go srv.Serve(lis)

	rawConn, err := net.Dial(lis.Addr().Network(), lis.Addr().String())
	require.NoError(t, err)
	defer rawConn.Close()

	// source=127.0.0.3:12345 destination=127.0.0.2:42
	sampleIPv4Addresses := []byte{0x7F, 0x00, 0x00, 0x03, 0x7F, 0x00, 0x00, 0x02, 0x30, 0x39, 0x00, 0x2A}
	// {0x21, 0x11, 0x00, 0x0C} - 4 bits version, 4 bits command, 4 bits address family, 4 bits protocol, 16 bits length
	sampleProxyV2Line := bytes.Join([][]byte{multiplexer.ProxyV2Prefix, {0x21, 0x11, 0x00, 0x0C}, sampleIPv4Addresses}, nil)

	require.NotEqual(t, "127.0.0.3:12345", rawConn.LocalAddr().String())
	require.NotEqual(t, "127.0.0.2:42", rawConn.RemoteAddr().String())

	_, err = rawConn.Write(sampleProxyV2Line)
	require.NoError(t, err)

	wrappedConn := <-dispatchC
	defer wrappedConn.Close()
	require.Equal(t, "127.0.0.2:42", wrappedConn.LocalAddr().String())
	require.Equal(t, "127.0.0.3:12345", wrappedConn.RemoteAddr().String())
}

type dispatchCredentials struct {
	credentials.TransportCredentials
	dispatchC chan net.Conn
}

// ServerHandshake implements [credentials.TransportCredentials].
func (d dispatchCredentials) ServerHandshake(c net.Conn) (net.Conn, credentials.AuthInfo, error) {
	d.dispatchC <- c
	return nil, nil, credentials.ErrConnDispatched
}
