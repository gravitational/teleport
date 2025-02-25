/*
Copyright 2021 Gravitational, Inc.

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

package client

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

type mockJoinServiceServer struct {
	proto.UnimplementedJoinServiceServer
	registerUsingTPMMethod    func(srv proto.JoinService_RegisterUsingTPMMethodServer) error
	registerUsingOracleMethod func(srv proto.JoinService_RegisterUsingOracleMethodServer) error
}

func (m *mockJoinServiceServer) RegisterUsingTPMMethod(srv proto.JoinService_RegisterUsingTPMMethodServer) error {
	return m.registerUsingTPMMethod(srv)
}

func (m *mockJoinServiceServer) RegisterUsingOracleMethod(srv proto.JoinService_RegisterUsingOracleMethodServer) error {
	return m.registerUsingOracleMethod(srv)
}

func TestJoinServiceClient_RegisterUsingTPMMethod(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	lis := bufconn.Listen(100)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close())
	})

	mockInitReq := &proto.RegisterUsingTPMMethodInitialRequest{
		JoinRequest: &types.RegisterUsingTokenRequest{
			Token: "token",
		},
	}
	mockChallenge := &proto.TPMEncryptedCredential{
		CredentialBlob: []byte("cred-blob"),
		Secret:         []byte("secret"),
	}
	mockChallengeResp := &proto.RegisterUsingTPMMethodChallengeResponse{
		Solution: []byte("solution"),
	}
	mockCerts := &proto.Certs{
		TLS: []byte("cert"),
	}
	mockService := &mockJoinServiceServer{
		registerUsingTPMMethod: func(srv proto.JoinService_RegisterUsingTPMMethodServer) error {
			req, err := srv.Recv()
			if !assert.NoError(t, err) {
				return err
			}
			assert.Empty(t, cmp.Diff(req.GetInit(), mockInitReq))

			err = srv.Send(&proto.RegisterUsingTPMMethodResponse{
				Payload: &proto.RegisterUsingTPMMethodResponse_ChallengeRequest{
					ChallengeRequest: mockChallenge,
				},
			})
			if !assert.NoError(t, err) {
				return err
			}

			req, err = srv.Recv()
			if !assert.NoError(t, err) {
				return err
			}
			assert.Empty(t, cmp.Diff(req.GetChallengeResponse(), mockChallengeResp))

			err = srv.Send(&proto.RegisterUsingTPMMethodResponse{
				Payload: &proto.RegisterUsingTPMMethodResponse_Certs{
					Certs: mockCerts,
				},
			})
			if !assert.NoError(t, err) {
				return err
			}
			return nil
		},
	}
	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})
	proto.RegisterJoinServiceServer(srv, mockService)

	go func() {
		defer cancel()
		err := srv.Serve(lis)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			assert.NoError(t, err)
		}
	}()

	// grpc.NewClient attempts to DNS resolve addr, whereas grpc.Dial doesn't.
	c, err := grpc.Dial(
		"bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	joinClient := NewJoinServiceClient(proto.NewJoinServiceClient(c))

	certs, err := joinClient.RegisterUsingTPMMethod(
		ctx,
		mockInitReq,
		func(challenge *proto.TPMEncryptedCredential) (*proto.RegisterUsingTPMMethodChallengeResponse, error) {
			assert.Empty(t, cmp.Diff(mockChallenge, challenge))
			return mockChallengeResp, nil
		},
	)
	if assert.NoError(t, err) {
		assert.Empty(t, cmp.Diff(mockCerts, certs))
	}
}

func TestJoinServiceClient_RegisterUsingOracleMethod(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	lis := bufconn.Listen(100)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close())
	})

	tokenReq := &types.RegisterUsingTokenRequest{
		Token: "token",
	}
	mockTokenRequest := &proto.RegisterUsingOracleMethodRequest{
		Request: &proto.RegisterUsingOracleMethodRequest_RegisterUsingTokenRequest{
			RegisterUsingTokenRequest: tokenReq,
		},
	}
	mockChallenge := "challenge"
	oracleReq := &proto.OracleSignedRequest{
		Headers: map[string]string{
			"x-teleport-challenge": mockChallenge,
		},
		PayloadHeaders: map[string]string{
			"x-teleport-challenge": mockChallenge,
		},
	}

	mockOracleRequest := &proto.RegisterUsingOracleMethodRequest{
		Request: &proto.RegisterUsingOracleMethodRequest_OracleRequest{
			OracleRequest: oracleReq,
		},
	}
	mockCerts := &proto.Certs{
		TLS: []byte("cert"),
	}
	mockService := &mockJoinServiceServer{
		registerUsingOracleMethod: func(srv proto.JoinService_RegisterUsingOracleMethodServer) error {
			tokenReq, err := srv.Recv()
			if !assert.NoError(t, err) {
				return err
			}
			assert.Empty(t, cmp.Diff(mockTokenRequest, tokenReq))
			err = srv.Send(&proto.RegisterUsingOracleMethodResponse{
				Response: &proto.RegisterUsingOracleMethodResponse_Challenge{
					Challenge: mockChallenge,
				},
			})
			if !assert.NoError(t, err) {
				return err
			}
			headerReq, err := srv.Recv()
			if !assert.NoError(t, err) {
				return err
			}
			assert.Empty(t, cmp.Diff(mockOracleRequest, headerReq))

			err = srv.Send(&proto.RegisterUsingOracleMethodResponse{
				Response: &proto.RegisterUsingOracleMethodResponse_Certs{
					Certs: mockCerts,
				},
			})
			if !assert.NoError(t, err) {
				return err
			}
			return nil
		},
	}
	srv := grpc.NewServer()
	t.Cleanup(srv.Stop)
	proto.RegisterJoinServiceServer(srv, mockService)

	go func() {
		defer cancel()
		err := srv.Serve(lis)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			assert.NoError(t, err)
		}
	}()

	// grpc.NewClient attempts to DNS resolve addr, whereas grpc.Dial doesn't.
	c, err := grpc.Dial(
		"bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	joinClient := NewJoinServiceClient(proto.NewJoinServiceClient(c))
	certs, err := joinClient.RegisterUsingOracleMethod(
		ctx,
		tokenReq,
		func(challenge string) (*proto.OracleSignedRequest, error) {
			assert.Equal(t, mockChallenge, challenge)
			return oracleReq, nil
		},
	)
	if assert.NoError(t, err) {
		assert.Empty(t, cmp.Diff(mockCerts, certs))
	}
}
