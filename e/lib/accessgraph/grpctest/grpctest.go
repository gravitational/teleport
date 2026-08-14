package grpctest

import (
	"context"
	"io"

	"google.golang.org/grpc"
)

// NewStreams creates a new bidirectional streaming gRPC pair, a bidirectional
// streaming client and server, for the given request type T1 and response type
// T2.
//
// The streams are directly connected without the use of network and are
// therefore suitable to be used with synctest.
//
// The client sends its requests on the clientStream which are directly received
// by the server via a channel with buffer size 1. The server sends its
// responses on its serverStream via another channel with buffer size 1.
func NewStreams[T1, T2 any](ctx context.Context) (grpc.BidiStreamingClient[T1, T2], grpc.BidiStreamingServer[T1, T2]) {
	t := tester[T1, T2]{
		ctx:      ctx,
		toServer: make(chan *T1, 1),
		toClient: make(chan *T2, 1),
	}
	return t.newClientStream(), t.newServerStream()
}

type tester[T1, T2 any] struct {
	ctx      context.Context
	toServer chan *T1
	toClient chan *T2
}

func (t *tester[T1, T2]) newClientStream() grpc.BidiStreamingClient[T1, T2] {
	return &client[T1, T2]{
		ctx:      t.ctx,
		toServer: t.toServer,
		toClient: t.toClient,
	}
}

func (t *tester[T1, T2]) newServerStream() grpc.BidiStreamingServer[T1, T2] {
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
