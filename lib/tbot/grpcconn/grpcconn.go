/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package grpcconn

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errUninitialized is returned when neither a connection or error has been set
// on the ClientConn. It's the "default" state of an uninitialized connection.
var errUninitialized = status.Error(codes.Unavailable, "gRPC client connection is uninitialized")

// ClientConn wraps a *grpc.ClientConn to make it possible to create and pass
// around gRPC client stubs before the connection has been properly established.
//
// This is useful in cases where we weren't able to connect and authenticate
// with the auth server on-startup (e.g. due to a network partition) but might
// be able to in the future, and it's more useful for tbot to run in a "degraded"
// state than to exit completely.
//
// The zero-value of a ClientConn returns an error with a gRPC Unavailable
// status code on each RPC.
type ClientConn struct {
	mu       sync.RWMutex
	err      error
	realConn *grpc.ClientConn
}

// WithConnection creates a ClientConn that will use the given connection for
// each RPC.
func WithConnection(conn *grpc.ClientConn) *ClientConn {
	return &ClientConn{realConn: conn}
}

// WithError creates ClientConn that will return the given error on each RPC
// until the the connection is set with SetConnection.
func WithError(err error) *ClientConn {
	return &ClientConn{err: err}
}

// SetConnection sets the connection that will be used for each RPC and discards
// any previously-set error.
func (c *ClientConn) SetConnection(conn *grpc.ClientConn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.realConn = conn
	c.err = nil
}

// SetError sets the error that will be returned from each RPC and discards any
// previously-set client.
func (c *ClientConn) SetError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.realConn = nil
	c.err = err
}

// Invoke implements grpc.ClientConnInterface.
func (c *ClientConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.realConn != nil {
		return c.realConn.Invoke(ctx, method, args, reply, opts...)
	}

	if c.err != nil {
		return c.err
	}

	return errUninitialized
}

// NewStream implements grpc.ClientConnInterface.
func (c *ClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.realConn != nil {
		return c.realConn.NewStream(ctx, desc, method, opts...)
	}

	if c.err != nil {
		return nil, c.err
	}

	return nil, errUninitialized
}

var _ grpc.ClientConnInterface = (*ClientConn)(nil)
