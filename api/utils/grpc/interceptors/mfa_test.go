// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interceptors_test

import (
	"context"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/testhelpers/mtls"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
)

const otpTestCode = "otp-test-code"

type mfaService struct {
	proto.UnimplementedAuthServiceServer
}

func (s *mfaService) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	if err := verifyMFAFromContext(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.PingResponse{}, nil
}

func verifyMFAFromContext(ctx context.Context) error {
	mfaResp, err := mfa.CredentialsFromContext(ctx)
	if err != nil {
		// (In production consider logging err, so we don't swallow it silently.)
		return trace.Wrap(&mfa.ErrAdminActionMFARequired)
	}

	switch r := mfaResp.Response.(type) {
	case *proto.MFAAuthenticateResponse_TOTP:
		if r.TOTP.Code != otpTestCode {
			return trace.AccessDenied("failed MFA verification")
		}
	default:
		return trace.BadParameter("unexpected mfa response type %T", r)
	}

	return nil
}

// TestGRPCErrorWrapping tests the error wrapping capability of the client
// and server unary and stream interceptors
func TestRetryWithMFA(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mtlsConfig := mtls.NewConfig(t)
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(mtlsConfig.ServerTLS)),
		grpc.ChainUnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)
	proto.RegisterAuthServiceServer(server, &mfaService{})
	go func() {
		server.Serve(listener)
	}()
	defer server.Stop()

	t.Run("without interceptor", func(t *testing.T) {
		conn, err := grpc.Dial(
			listener.Addr().String(),
			grpc.WithTransportCredentials(credentials.NewTLS(mtlsConfig.ClientTLS)),
			grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
		)
		require.NoError(t, err)
		defer conn.Close()

		client := proto.NewAuthServiceClient(conn)
		_, err = client.Ping(context.Background(), &proto.PingRequest{})
		assert.ErrorIs(t, err, &mfa.ErrAdminActionMFARequired, "Ping error mismatch")
	})

	okMFAClient := &fakeMFACeremonyClient{}

	mfaCeremonyErr := trace.BadParameter("client does not support mfa")
	nokMFAClient := &fakeMFACeremonyClient{
		ceremonyErr: mfaCeremonyErr,
	}

	t.Run("with interceptor", func(t *testing.T) {
		t.Run("ok mfa ceremony", func(t *testing.T) {
			conn, err := grpc.Dial(
				listener.Addr().String(),
				grpc.WithTransportCredentials(credentials.NewTLS(mtlsConfig.ClientTLS)),
				grpc.WithChainUnaryInterceptor(
					interceptors.WithMFAUnaryInterceptor(okMFAClient),
					interceptors.GRPCClientUnaryErrorInterceptor,
				),
			)
			require.NoError(t, err)
			defer conn.Close()

			client := proto.NewAuthServiceClient(conn)
			_, err = client.Ping(ctx, &proto.PingRequest{})
			assert.NoError(t, err)
		})

		t.Run("nok mfa ceremony", func(t *testing.T) {
			conn, err := grpc.Dial(
				listener.Addr().String(),
				grpc.WithTransportCredentials(credentials.NewTLS(mtlsConfig.ClientTLS)),
				grpc.WithChainUnaryInterceptor(
					interceptors.WithMFAUnaryInterceptor(nokMFAClient),
					interceptors.GRPCClientUnaryErrorInterceptor,
				),
			)
			require.NoError(t, err)
			defer conn.Close()

			client := proto.NewAuthServiceClient(conn)
			_, err = client.Ping(ctx, &proto.PingRequest{})
			assert.ErrorIs(t, err, &mfa.ErrAdminActionMFARequired, "Ping error mismatch")
			assert.ErrorIs(t, err, mfaCeremonyErr, "Ping error mismatch")
		})

		t.Run("ok mfa in context", func(t *testing.T) {
			conn, err := grpc.Dial(
				listener.Addr().String(),
				grpc.WithTransportCredentials(credentials.NewTLS(mtlsConfig.ClientTLS)),
				grpc.WithChainUnaryInterceptor(
					interceptors.WithMFAUnaryInterceptor(okMFAClient),
					interceptors.GRPCClientUnaryErrorInterceptor,
				),
			)
			require.NoError(t, err)
			defer conn.Close()

			mfaResp, _ := okMFAClient.PromptMFA(ctx, nil)
			ctx := mfa.ContextWithMFAResponse(ctx, mfaResp)

			client := proto.NewAuthServiceClient(conn)
			_, err = client.Ping(ctx, &proto.PingRequest{})
			assert.NoError(t, err)
		})
	})
}

type fakeMFACeremonyClient struct {
	ceremonyErr error
}

func (c *fakeMFACeremonyClient) CreateAuthenticateChallenge(ctx context.Context, in *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	return &proto.MFAAuthenticateChallenge{}, nil
}

func (c *fakeMFACeremonyClient) PromptMFA(ctx context.Context, chal *proto.MFAAuthenticateChallenge, promptOpts ...mfa.PromptOpt) (*proto.MFAAuthenticateResponse, error) {
	if c.ceremonyErr != nil {
		return nil, c.ceremonyErr
	}

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: otpTestCode,
			},
		},
	}, nil
}
