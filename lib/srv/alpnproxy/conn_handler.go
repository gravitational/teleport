/*
Copyright 2022 Gravitational, Inc.

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

package alpnproxy

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/gravitational/trace"
)

// connectionHandlerOptions contains options when ALPN server handles an
// incoming connection.
type connectionHandlerOptions struct {
	// waitForAsyncHandlers makes the connection handler wait until the
	// connection is closed for async handlers.
	waitForAsyncHandlers bool
	// defaultTLSConfig is the default TLS config served to the incoming
	// connection during TLS handshake, if HandlerDesc does not provide a
	// tls.Config.
	defaultTLSConfig *tls.Config
}

// ConnectionHandlerOption defines an option function for specifying connection
// handler options.
type ConnectionHandlerOption func(*connectionHandlerOptions)

// WithWaitForAsyncHandlers is an option function that makes the server wait
// for async handlers to close the connections.
func WithWaitForAsyncHandlers() ConnectionHandlerOption {
	return func(opt *connectionHandlerOptions) {
		opt.waitForAsyncHandlers = true
	}
}

// WithDefaultTLSconfig is an option function that provides a default TLS
// config.
func WithDefaultTLSconfig(tlsConfig *tls.Config) ConnectionHandlerOption {
	return func(opt *connectionHandlerOptions) {
		opt.defaultTLSConfig = tlsConfig
	}
}

// ConnectionHandler defines a function for serving incoming connections.
type ConnectionHandler func(ctx context.Context, conn net.Conn) error

// ConnectionHandlerWrapper is a wrapper of ConnectionHandler. This wrapper is
// mainly used as a placeholder to resolve circular dependencies. Therefore, it
// is important to use pointer receivers when defining its functions to make
// sure the updated ConnectionHandler reference is used.
type ConnectionHandlerWrapper struct {
	h ConnectionHandler
}

// Set updates inner ConnectionHandler to use.
func (w *ConnectionHandlerWrapper) Set(h ConnectionHandler) {
	w.h = h
}

// HandleConnection implements ConnectionHandler.
func (w *ConnectionHandlerWrapper) HandleConnection(ctx context.Context, conn net.Conn) error {
	if w.h == nil {
		return trace.NotFound("missing ConnectionHandler")
	}
	return w.h(ctx, conn)
}
