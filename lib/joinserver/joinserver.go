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

// Package joinserver contains the implementation of the JoinService gRPC server
// which runs on both Auth and Proxy.
package joinserver

import (
	"context"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"
)

type joinServiceClient interface {
	RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterChallengeResponseFunc) (*proto.Certs, error)
}

// JoinServiceGRPCServer implements proto.JoinServiceServer and is designed
// to run on both the Teleport Proxy and Auth servers.
type JoinServiceGRPCServer struct {
	joinServiceClient joinServiceClient
}

// NewJoinGRPCServer returns a new JoinServiceGRPCServer.
func NewJoinServiceGRPCServer(joinServiceClient joinServiceClient) *JoinServiceGRPCServer {
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
	ctx := srv.Context()

	// call RegisterUsingIAMMethod with a callback to get the challenge response
	// from the gRPC client
	certs, err := s.joinServiceClient.RegisterUsingIAMMethod(ctx, func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
		// first forward challenge from to the client
		err := srv.Send(&proto.RegisterUsingIAMMethodResponse{
			Challenge: challenge,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// then get the response from the client and return it
		req, err := srv.Recv()
		return req, trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// finally, send the certs on the response stream
	return trace.Wrap(srv.Send(&proto.RegisterUsingIAMMethodResponse{
		Certs: certs,
	}))
}
