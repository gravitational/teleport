// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/utils"
)

// ConnContextForWebServer implements the http.Server.ConnContext when creating
// the web server.
func ConnContextForWebServer(ctx context.Context, conn net.Conn) context.Context {
	// Set connection addresses.
	ctx = utils.ClientAddrContext(ctx, conn.RemoteAddr(), conn.LocalAddr())

	// Set a flag on whether the addresses come from the multiplexer.
	ctx = withClientAddrFromMultiplexer(ctx, conn)
	return ctx
}

func withClientAddrFromMultiplexer(ctx context.Context, conn net.Conn) context.Context {
	if tlsConn, ok := conn.(*tls.Conn); ok {
		conn = tlsConn.NetConn()
	}

	multiplexConn, ok := conn.(*multiplexer.Conn)
	if ok && multiplexConn.HasProxyLine() {
		return utils.AddFlagToContext[clientAddrFromMultiplexer](ctx)
	}
	return ctx
}
func hasClientAddrFromMultiplexer(ctx context.Context) bool {
	return utils.GetFlagFromContext[clientAddrFromMultiplexer](ctx)
}

type clientAddrFromMultiplexer struct{}
