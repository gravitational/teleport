/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Package joinserver contains the implementation of the JoinService gRPC server
// which runs on both Auth and Proxy.
package joinserver

import (
	"context"
	"net"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/peer"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	iamJoinRequestTimeout   = time.Minute
	azureJoinRequestTimeout = time.Minute
)

type joinServiceClient interface {
	RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterIAMChallengeResponseFunc) (*proto.Certs, error)
	RegisterUsingAzureMethod(ctx context.Context, challengeResponse client.RegisterAzureChallengeResponseFunc) (*proto.Certs, error)
}

// JoinServiceGRPCServer implements proto.JoinServiceServer and is designed
// to run on both the Teleport Proxy and Auth servers.
type JoinServiceGRPCServer struct {
	joinServiceClient joinServiceClient
	clock             clockwork.Clock
}

// NewJoinServiceGRPCServer returns a new JoinServiceGRPCServer.
func NewJoinServiceGRPCServer(joinServiceClient joinServiceClient) *JoinServiceGRPCServer {
	return &JoinServiceGRPCServer{
		joinServiceClient: joinServiceClient,
		clock:             clockwork.NewRealClock(),
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

	// Enforce a timeout on the entire RPC so that misbehaving clients cannot
	// hold connections open indefinitely.
	timeout := s.clock.After(iamJoinRequestTimeout)

	// The only way to cancel a blocked Send or Recv on the server side without
	// adding an interceptor to the entire gRPC service is to return from the
	// handler https://github.com/grpc/grpc-go/issues/465#issuecomment-179414474
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.registerUsingIAMMethod(ctx, srv)
	}()
	select {
	case err := <-errCh:
		// Completed before the deadline, return the error (may be nil).
		return trace.Wrap(err)
	case <-timeout:
		nodeAddr := ""
		if peerInfo, ok := peer.FromContext(ctx); ok {
			nodeAddr = peerInfo.Addr.String()
		}
		logrus.Warnf("IAM join attempt timed out, node at (%s) is misbehaving or did not close the connection after encountering an error.", nodeAddr)
		// Returning here should cancel any blocked Send or Recv operations.
		return trace.LimitExceeded("RegisterUsingIAMMethod timed out after %s, terminating the stream on the server", iamJoinRequestTimeout)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

func (s *JoinServiceGRPCServer) registerUsingIAMMethod(ctx context.Context, srv proto.JoinService_RegisterUsingIAMMethodServer) error {
	// Call RegisterUsingIAMMethod with a callback to get the challenge response
	// from the gRPC client.
	certs, err := s.joinServiceClient.RegisterUsingIAMMethod(ctx, func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
		// First, forward the challenge from Auth to the client.
		err := srv.Send(&proto.RegisterUsingIAMMethodResponse{
			Challenge: challenge,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Then get the response from the client and return it.
		req, err := srv.Recv()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := setClientRemoteAddr(ctx, req.RegisterUsingTokenRequest); err != nil {
			return nil, trace.Wrap(err)
		}

		return req, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// finally, send the certs on the response stream
	return trace.Wrap(srv.Send(&proto.RegisterUsingIAMMethodResponse{
		Certs: certs,
	}))
}

// RegisterUsingAzureMethod allows nodes and proxies to join the cluster using the
// Azure join method.
//
// The server will generate a base64-encoded crypto-random challenge and
// send it on the server stream. The caller is expected to respond on
// the client stream with a RegisterUsingTokenRequest including a signed
// attested data document with the challenge string. Finally, the signed
// cluster certs are sent on the server stream.
func (s *JoinServiceGRPCServer) RegisterUsingAzureMethod(srv proto.JoinService_RegisterUsingAzureMethodServer) error {
	ctx := srv.Context()

	// Enforce a timeout on the entire RPC so that misbehaving clients cannot
	// hold connections open indefinitely.
	timeout := s.clock.After(azureJoinRequestTimeout)

	// The only way to cancel a blocked Send or Recv on the server side without
	// adding an interceptor to the entire gRPC service is to return from the
	// handler https://github.com/grpc/grpc-go/issues/465#issuecomment-179414474
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.registerUsingAzureMethod(ctx, srv)
	}()
	select {
	case err := <-errCh:
		// Completed before the deadline, return the error (may be nil).
		return trace.Wrap(err)
	case <-timeout:
		nodeAddr := ""
		if peerInfo, ok := peer.FromContext(ctx); ok {
			nodeAddr = peerInfo.Addr.String()
		}
		logrus.Warnf("Azure join attempt timed out, node at (%s) is misbehaving or did not close the connection after encountering an error.", nodeAddr)
		// Returning here should cancel any blocked Send or Recv operations.
		return trace.LimitExceeded("RegisterUsingAzureMethod timed out after %s, terminating the stream on the server", azureJoinRequestTimeout)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

func checkForProxyRole(identity tlsca.Identity) bool {
	const proxyRole = string(types.RoleProxy)
	return slices.Contains(identity.Groups, proxyRole) || slices.Contains(identity.SystemRoles, proxyRole)
}

func setClientRemoteAddr(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	// If request is coming from the Proxy, trust the IP set on the request.
	if user, err := authz.UserFromContext(ctx); err == nil && checkForProxyRole(user.GetIdentity()) {
		return nil
	}
	// Otherwise this is (likely) the proxy, set the IP from the connection.
	p, ok := peer.FromContext(ctx)
	if !ok {
		return trace.BadParameter("could not get peer from the context")
	}
	req.RemoteAddr = p.Addr.String() // Addr without port is used in tests.
	if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		req.RemoteAddr = ip
	}
	return nil
}

func (s *JoinServiceGRPCServer) registerUsingAzureMethod(ctx context.Context, srv proto.JoinService_RegisterUsingAzureMethodServer) error {
	certs, err := s.joinServiceClient.RegisterUsingAzureMethod(ctx, func(challenge string) (*proto.RegisterUsingAzureMethodRequest, error) {
		err := srv.Send(&proto.RegisterUsingAzureMethodResponse{
			Challenge: challenge,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		req, err := srv.Recv()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := setClientRemoteAddr(ctx, req.RegisterUsingTokenRequest); err != nil {
			return nil, trace.Wrap(err)
		}

		return req, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(srv.Send(&proto.RegisterUsingAzureMethodResponse{
		Certs: certs,
	}))
}
