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
	"log/slog"
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
	tpmJoinRequestTimeout   = time.Minute
)

type joinServiceClient interface {
	RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterIAMChallengeResponseFunc) (*proto.Certs, error)
	RegisterUsingAzureMethod(ctx context.Context, challengeResponse client.RegisterAzureChallengeResponseFunc) (*proto.Certs, error)
	RegisterUsingTPMMethod(
		ctx context.Context,
		initReq *proto.RegisterUsingTPMMethodInitialRequest,
		solveChallenge client.RegisterTPMChallengeResponseFunc,
	) (*proto.Certs, error)
}

// JoinServiceGRPCServer implements proto.JoinServiceServer and is designed
// to run on both the Teleport Proxy and Auth servers.
//
// On the Proxy, this uses a gRPC client to forward the request to the Auth
// server. On the Auth Server, this is passed to auth.ServerWithRoles and
// through to auth.Server to be handled.
type JoinServiceGRPCServer struct {
	*proto.UnimplementedJoinServiceServer

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
	// Enforce a timeout on the entire RPC so that misbehaving clients cannot
	// hold connections open indefinitely.
	timeout := s.clock.NewTimer(iamJoinRequestTimeout)
	defer timeout.Stop()

	// The only way to cancel a blocked Send or Recv on the server side without
	// adding an interceptor to the entire gRPC service is to return from the
	// handler https://github.com/grpc/grpc-go/issues/465#issuecomment-179414474
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.registerUsingIAMMethod(srv)
	}()
	select {
	case err := <-errCh:
		// Completed before the deadline, return the error (may be nil).
		return trace.Wrap(err)
	case <-timeout.Chan():
		nodeAddr := ""
		if peerInfo, ok := peer.FromContext(srv.Context()); ok {
			nodeAddr = peerInfo.Addr.String()
		}
		logrus.Warnf("IAM join attempt timed out, node at (%s) is misbehaving or did not close the connection after encountering an error.", nodeAddr)
		// Returning here should cancel any blocked Send or Recv operations.
		return trace.LimitExceeded("RegisterUsingIAMMethod timed out after %s, terminating the stream on the server", iamJoinRequestTimeout)
	}
}

func (s *JoinServiceGRPCServer) registerUsingIAMMethod(srv proto.JoinService_RegisterUsingIAMMethodServer) error {
	ctx := srv.Context()
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
	// Enforce a timeout on the entire RPC so that misbehaving clients cannot
	// hold connections open indefinitely.
	timeout := s.clock.NewTimer(azureJoinRequestTimeout)
	defer timeout.Stop()

	// The only way to cancel a blocked Send or Recv on the server side without
	// adding an interceptor to the entire gRPC service is to return from the
	// handler https://github.com/grpc/grpc-go/issues/465#issuecomment-179414474
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.registerUsingAzureMethod(srv)
	}()
	select {
	case err := <-errCh:
		// Completed before the deadline, return the error (may be nil).
		return trace.Wrap(err)
	case <-timeout.Chan():
		nodeAddr := ""
		if peerInfo, ok := peer.FromContext(srv.Context()); ok {
			nodeAddr = peerInfo.Addr.String()
		}
		logrus.Warnf("Azure join attempt timed out, node at (%s) is misbehaving or did not close the connection after encountering an error.", nodeAddr)
		// Returning here should cancel any blocked Send or Recv operations.
		return trace.LimitExceeded("RegisterUsingAzureMethod timed out after %s, terminating the stream on the server", azureJoinRequestTimeout)
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

func (s *JoinServiceGRPCServer) registerUsingAzureMethod(srv proto.JoinService_RegisterUsingAzureMethodServer) error {
	ctx := srv.Context()
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

// RegisterUsingTPMMethod allows nodes and bots to join the cluster using the
// TPM join method.
//
// When running on the Auth server, this method will call the
// auth.ServerWithRoles's RegisterUsingTPMMethod method. When running on the
// Proxy, this method will forward the request to the Auth server.
func (s *JoinServiceGRPCServer) RegisterUsingTPMMethod(srv proto.JoinService_RegisterUsingTPMMethodServer) error {
	ctx := srv.Context()

	// Enforce a timeout on the entire RPC so that misbehaving clients cannot
	// hold connections open indefinitely.
	timeout := s.clock.After(tpmJoinRequestTimeout)

	// The only way to cancel a blocked Send or Recv on the server side without
	// adding an interceptor to the entire gRPC service is to return from the
	// handler https://github.com/grpc/grpc-go/issues/465#issuecomment-179414474
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.registerUsingTPMMethod(ctx, srv)
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
		slog.WarnContext(
			srv.Context(),
			"TPM join attempt timed out, node is misbehaving or did not close the connection after encountering an error",
			"node_addr", nodeAddr,
		)
		// Returning here should cancel any blocked Send or Recv operations.
		return trace.LimitExceeded(
			"RegisterUsingTPMMethod timed out after %s, terminating the stream on the server",
			tpmJoinRequestTimeout,
		)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

func (s *JoinServiceGRPCServer) registerUsingTPMMethod(
	ctx context.Context, srv proto.JoinService_RegisterUsingTPMMethodServer,
) error {
	// Get initial payload from the client
	req, err := srv.Recv()
	if err != nil {
		return trace.Wrap(err, "receiving initial payload")
	}
	initReq := req.GetInit()
	if initReq == nil {
		return trace.BadParameter("expected non-nil Init payload")
	}
	if initReq.JoinRequest == nil {
		return trace.BadParameter(
			"expected JoinRequest in RegisterUsingTPMMethodRequest_Init, got nil",
		)
	}
	if err := setClientRemoteAddr(ctx, initReq.JoinRequest); err != nil {
		return trace.Wrap(err, "setting client address")
	}

	certs, err := s.joinServiceClient.RegisterUsingTPMMethod(
		ctx,
		initReq,
		func(challenge *proto.TPMEncryptedCredential,
		) (*proto.RegisterUsingTPMMethodChallengeResponse, error) {
			// First, forward the challenge from Auth to the client.
			err := srv.Send(&proto.RegisterUsingTPMMethodResponse{
				Payload: &proto.RegisterUsingTPMMethodResponse_ChallengeRequest{
					ChallengeRequest: challenge,
				},
			})
			if err != nil {
				return nil, trace.Wrap(
					err, "forwarding challenge to client",
				)
			}
			// Get response from Client
			req, err := srv.Recv()
			if err != nil {
				return nil, trace.Wrap(
					err, "receiving challenge solution from client",
				)
			}
			challengeResponse := req.GetChallengeResponse()
			if challengeResponse == nil {
				return nil, trace.BadParameter(
					"expected non-nil ChallengeResponse payload",
				)
			}
			return challengeResponse, nil
		})
	if err != nil {
		return trace.Wrap(err)
	}

	// finally, send the certs on the response stream
	return trace.Wrap(srv.Send(&proto.RegisterUsingTPMMethodResponse{
		Payload: &proto.RegisterUsingTPMMethodResponse_Certs{
			Certs: certs,
		},
	}))
}
