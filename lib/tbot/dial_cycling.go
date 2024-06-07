/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"io"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
)

type cyclingInner struct {
	conn   *grpc.ClientConn
	active atomic.Int32
}

type clientStreamWithFinalizer struct {
	grpc.ClientStream
}

type grpcClientConnInterfaceCloser = interface {
	grpc.ClientConnInterface
	io.Closer
}

func newDialCycling(
	cycleCount int32,
) func(
	ctx context.Context, target string, opts ...grpc.DialOption,
) (grpcClientConnInterfaceCloser, error) {
	return func(
		ctx context.Context, target string, opts ...grpc.DialOption,
	) (grpcClientConnInterfaceCloser, error) {
		cc := &cyclingConn{
			target:     target,
			opts:       opts,
			cycleCount: cycleCount,
		}

		inner, err := cc.dial(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cc.inner = inner

		return cc, nil
	}
}

type cyclingConn struct {
	target     string
	opts       []grpc.DialOption
	cycleCount int32

	inner   *cyclingInner
	mu      sync.Mutex
	started int32
}

func (c *cyclingConn) dial(ctx context.Context) (*cyclingInner, error) {
	conn, err := grpc.DialContext(ctx, c.target, c.opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	inner := &cyclingInner{
		conn: conn,
	}
	inner.active.Add(1)

	return inner, nil
}

// Invoke implements [grpc.ClientConnInterface].
func (c *cyclingConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	c.mu.Lock()
	if c.inner == nil {
		i, err := c.dial(ctx)
		if err != nil {
			c.mu.Unlock()
			return err
		}
		c.inner = i
		c.started = 0
	}
	inner := c.inner
	c.started++
	if c.started >= c.cycleCount {
		c.inner = nil
	} else {
		inner.active.Add(1)
	}
	c.mu.Unlock()

	defer func() {
		if inner.active.Add(-1) <= 0 {
			_ = inner.conn.Close()
		}
	}()

	return inner.conn.Invoke(ctx, method, args, reply, opts...)
}

// NewStream implements [grpc.ClientConnInterface].
func (c *cyclingConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	c.mu.Lock()
	if c.inner == nil {
		i, err := c.dial(ctx)
		if err != nil {
			c.mu.Unlock()
			return nil, err
		}
		c.inner = i
		c.started = 0
	}
	inner := c.inner
	c.started++
	if c.started >= c.cycleCount {
		c.inner = nil
	} else {
		inner.active.Add(1)
	}
	c.mu.Unlock()

	cs, err := inner.conn.NewStream(ctx, desc, method, opts...)

	csf := &clientStreamWithFinalizer{cs}
	runtime.SetFinalizer(csf, func(*clientStreamWithFinalizer) {
		if inner.active.Add(-1) <= 0 {
			go inner.conn.Close()
		}
	})

	return csf, err
}

// Close implements [io.Closer].
func (c *cyclingConn) Close() error {
	return nil
}
