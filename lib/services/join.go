package services

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// JoinService abstracts the proto.JoinService interface so that it can by
// implemented by both the auth client and the auth server.
type JoinService interface {
	// RegisterUsingIAMMethod registers the caller using the IAM join method and
	// returns signed certs to join the cluster.
	//
	// The server will generate a base64-encoded crypto-random challenge and
	// send it on the challenge channel. The caller is expected to respond on
	// the request channel with a RegisterUsingTokenRequest including a signed
	// sts:GetCallerIdentity request with the challenge string.
	RegisterUsingIAMMethod(ctx context.Context, challenge chan<- string, request <-chan *types.RegisterUsingTokenRequest) (*proto.Certs, error)
}

// JoinServiceGRPCServer implements proto.JoinServiceServer and is designed
// to run on both the Teleport Proxy and Auth servers.
type JoinServiceGRPCServer struct {
	joinServiceClient JoinService
}

// NewJoinGRPCServer returns a new JoinServiceGRPCServer.
func NewJoinServiceGRPCServer(joinServiceClient JoinService) *JoinServiceGRPCServer {
	return &JoinServiceGRPCServer{
		joinServiceClient: joinServiceClient,
	}
}

// RegisterUsingIAMMethod allows nodes and proxies to join the cluster using the
// IAM join method.
//
// The server will generate a base64-encoded crypto-random challenge and
// send it on the server stream. The caller is expected to respond on
// the client stream with a RegisterUsingTokenRequest including a signed
// sts:GetCallerIdentity request with the challenge string. Finally, the signed
// cluster certs are sent on the server stream.
func (s *JoinServiceGRPCServer) RegisterUsingIAMMethod(srv proto.JoinService_RegisterUsingIAMMethodServer) error {
	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	challengeChan := make(chan string)
	reqChan := make(chan *types.RegisterUsingTokenRequest)
	errChan := make(chan error)

	// set up a goroutine to forward between the gRPC streams and the
	// JoinService channels
	go func() {
		defer close(errChan)

		// first forward challenge from auth to client
		select {
		case challenge := <-challengeChan:
			err := srv.Send(&proto.RegisterUsingIAMMethodResponse{
				Challenge: challenge,
			})
			if err != nil {
				cancel()
				errChan <- trace.Wrap(err)
				return
			}
		case <-ctx.Done():
			errChan <- trace.Wrap(ctx.Err())
			return
		}

		// then forward request from client to auth
		req, err := srv.Recv()
		if err != nil {
			cancel()
			errChan <- trace.Wrap(err)
			return
		}
		select {
		case reqChan <- req:
		case <-ctx.Done():
			errChan <- trace.Wrap(ctx.Err())
			return
		}
	}()

	// call the auth register method. This blocks, but if the forwarding
	// goroutine has an error, the context will be cancelled.
	certs, registerErr := s.joinServiceClient.RegisterUsingIAMMethod(ctx, challengeChan, reqChan)

	// block until the forwarding goroutine returns
	forwardingErr := <-errChan

	// return any errors
	if err := trace.NewAggregate(registerErr, forwardingErr); err != nil {
		return trace.Wrap(err)
	}

	// finally, send the certs on the response stream
	return trace.Wrap(srv.Send(&proto.RegisterUsingIAMMethodResponse{
		Certs: certs,
	}))
}
