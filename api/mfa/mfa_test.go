/*
Copyright 2023 Gravitational, Inc.

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

package mfa_test

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

// TestMFAPerRPCCredentials tests the MFA verification process between a client and server.
func TestMFAPerRPCCredentials(t *testing.T) {
	t.Parallel()

	mtlsConfig := mtls.NewConfig(t)
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.Creds(credentials.NewTLS(mtlsConfig.ServerTLS)),
	)
	proto.RegisterAuthServiceServer(server, &mfaService{})
	go func() {
		server.Serve(listener)
	}()
	defer server.Stop()

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

	mfaTestResp := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: otpTestCode,
			},
		},
	}

	_, err = client.Ping(context.Background(), &proto.PingRequest{}, mfa.WithCredentials(mfaTestResp))
	assert.NoError(t, err)
}
