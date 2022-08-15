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

// ConnectionHandler is an interface for serving incoming connections.
type ConnectionHandler interface {
	HandleConnection(ctx context.Context, conn net.Conn) error
}

// ConnectionHandlerFunc defines a function to serve incoming connections.
type ConnectionHandlerFunc func(ctx context.Context, conn net.Conn) error

// HandleConnection implements ConnectionHandler interface
func (f ConnectionHandlerFunc) HandleConnection(ctx context.Context, conn net.Conn) error {
	return f(ctx, conn)
}

// ConnectionHandlerWrapper is a wrapper of ConnectionHandler. Mainly used as a
// placeholder to resolve circular dependencies.
type ConnectionHandlerWrapper struct {
	ConnectionHandler
}

// Set updates inner ConnectionHandler to use.
func (w *ConnectionHandlerWrapper) Set(h ConnectionHandler) {
	w.ConnectionHandler = h
}

// HandleConnection implements ConnectionHandler.
func (w ConnectionHandlerWrapper) HandleConnection(ctx context.Context, conn net.Conn) error {
	if w.ConnectionHandler == nil {
		return trace.NotFound("missing ConnectionHandler")
	}
	return w.ConnectionHandler.HandleConnection(ctx, conn)
}
