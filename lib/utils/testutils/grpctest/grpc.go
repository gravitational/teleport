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

package grpctest

import (
	"context"
	"io"

	"google.golang.org/grpc"
)

// NewGRPCStreams creates a new bidirectional streaming gRPC pair, a bidirectional
// streaming client and server, for the given request type T1 and response type
// T2.
//
// The streams are directly connected without the use of network and are
// therefore suitable to be used with synctest.
//
// The client sends its requests on the clientStream which are directly received
// by the server via a channel with buffer size 1. The server sends its
// responses on its serverStream via another channel with buffer size 1.
//
// Private fields are purposefully written in a *not* concurrency-safe manner to
// simulate non-concurrency safety of real over-the-network GRPC stream. It will
// be caught when executing the test with the race detector enbled.
func NewGRPCTester[T1, T2 any](ctx context.Context) *GRPCTester[T1, T2] {
	return &GRPCTester[T1, T2]{
		ctx:      ctx,
		toServer: make(chan *T1, 1),
		toClient: make(chan *T2, 1),
	}
}

type GRPCTester[T1, T2 any] struct {
	ctx      context.Context
	toServer chan *T1
	toClient chan *T2
}

func (t *GRPCTester[T1, T2]) NewClientStream() grpc.BidiStreamingClient[T1, T2] {
	return &client[T1, T2]{
		ctx:      t.ctx,
		toServer: t.toServer,
		toClient: t.toClient,
	}
}

func (t *GRPCTester[T1, T2]) NewServerStream() grpc.BidiStreamingServer[T1, T2] {
	return &server[T1, T2]{
		ctx:      t.ctx,
		toServer: t.toServer,
		toClient: t.toClient,
	}
}

type client[T1, T2 any] struct {
	grpc.ClientStream
	ctx      context.Context
	toServer chan *T1
	toClient chan *T2
	// simulate non-concurrency safety
	sendRaceDetector    bool
	receiveRaceDetector bool
}

func (c *client[T1, T2]) Context() context.Context {
	return c.ctx
}

func (c *client[T1, T2]) Send(req *T1) error {
	c.sendRaceDetector = true // simulate non-concurrency safety
	select {
	case c.toServer <- req:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *client[T1, T2]) Recv() (*T2, error) {
	c.receiveRaceDetector = true // simulate non-concurrency safety
	select {
	case resp := <-c.toClient:
		return resp, nil
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

func (c *client[T1, T2]) CloseSend() error {
	close(c.toServer)
	return nil
}

type server[T1, T2 any] struct {
	grpc.ServerStream
	ctx      context.Context
	toServer chan *T1
	toClient chan *T2
	// simulate non-concurrency safety
	sendRaceDetector    bool
	receiveRaceDetector bool
}

func (s *server[T1, T2]) Context() context.Context {
	return s.ctx
}

func (s *server[T1, T2]) Send(resp *T2) error {
	s.sendRaceDetector = true
	select {
	case s.toClient <- resp:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *server[T1, T2]) Recv() (*T1, error) {
	s.receiveRaceDetector = true
	select {
	case req, ok := <-s.toServer:
		if !ok {
			return nil, io.EOF
		}
		return req, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}
