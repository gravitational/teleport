/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"net"

	"github.com/gravitational/trace"
)

type webContextKey string

const (
	// ClientSrcAddrContextKey is context key for client source address
	ClientSrcAddrContextKey webContextKey = "teleport-clientSrcAddrContextKey"
	// ClientDstAddrContextKey is context key for client destination address
	ClientDstAddrContextKey webContextKey = "teleport-clientDstAddrContextKey"
)

// ClientAddrContext is used by server that accepts connections from clients to set incoming
// client's connection source and destination addresses to the context, so it could be later used
// for IP propagation purpose. It is used when we don't have other source for client source/destination
// addresses (don't have direct access to net.Conn)
func ClientAddrContext(ctx context.Context, src net.Addr, dst net.Addr) context.Context {
	ctx = context.WithValue(ctx, ClientSrcAddrContextKey, src)
	return context.WithValue(ctx, ClientDstAddrContextKey, dst)
}

// ClientAddrFromContext gets client source address and destination addresses from the context. If an address is
// not present, nil will be returned
func ClientAddrFromContext(ctx context.Context) (src net.Addr, dst net.Addr) {
	if ctx == nil {
		return nil, nil
	}
	src, _ = ctx.Value(ClientSrcAddrContextKey).(net.Addr)
	dst, _ = ctx.Value(ClientDstAddrContextKey).(net.Addr)
	return
}

// ClientIPFromConn extracts host from provided remote address.
func ClientIPFromConn(conn net.Conn) (string, error) {
	clientRemoteAddr := conn.RemoteAddr()

	clientIP, _, err := net.SplitHostPort(clientRemoteAddr.String())
	if err != nil {
		return "", trace.Wrap(err)
	}

	return clientIP, nil
}
