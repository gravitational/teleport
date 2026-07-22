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
	"net"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/utils"
)

// PPV2ServerCredentials is a wrapper around a gRPC
// [credentials.TransportCredentials] that reads a PROXY protocol v2 header
// before passing the incoming connection to the wrapped credentials. When used
// as client credentials, the wrapped credentials are used as-is.
type PPV2ServerCredentials struct {
	_ struct{}

	credentials.TransportCredentials
}

var _ credentials.TransportCredentials = PPV2ServerCredentials{}

// Clone implements [credentials.TransportCredentials].
func (p PPV2ServerCredentials) Clone() credentials.TransportCredentials {
	return PPV2ServerCredentials{
		TransportCredentials: p.TransportCredentials.Clone(),
	}
}

// ServerHandshake implements [credentials.TransportCredentials].
func (p PPV2ServerCredentials) ServerHandshake(nc net.Conn) (net.Conn, credentials.AuthInfo, error) {
	proxyLine, err := multiplexer.ReadProxyLineV2(nc)
	if err != nil {
		_ = nc.Close()
		return nil, nil, trace.Wrap(err)
	}
	if proxyLine != nil {
		nc = utils.NewConnWithAddr(nc, &proxyLine.Destination, &proxyLine.Source)
	}
	return p.TransportCredentials.ServerHandshake(nc)
}
