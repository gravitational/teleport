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

package client

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
)

const (
	proxyAddr   = "test-proxy"
	otpTestCode = "otp-test-code"
)

type mfaService struct {
	*proto.UnimplementedAuthServiceServer
}

func (s *mfaService) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{
		ProxyPublicAddr: proxyAddr,
	}, nil
}

func (s *mfaService) CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	return &proto.MFAAuthenticateChallenge{
		TOTP: &proto.TOTPChallenge{},
	}, nil
}

func TestPerformMFACeremony(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	server := startMockServer(t, &mfaService{})

	mfaTestResp := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: otpTestCode,
			},
		},
	}

	cfg := server.clientCfg()
	cfg.MFAPromptConstructor = func(opts ...mfa.PromptOpt) mfa.Prompt {
		return &fakeMFAPrompt{mfaTestResp}
	}

	clt, err := New(ctx, cfg)
	require.NoError(t, err)

	resp, err := clt.performAdminActionMFACeremony(ctx)
	require.NoError(t, err)
	require.Equal(t, mfaTestResp.Response, resp.Response)
}

type fakeMFAPrompt struct {
	totpResp *proto.MFAAuthenticateResponse
}

func (p *fakeMFAPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if chal.TOTP != nil {
		return p.totpResp, nil
	}
	return nil, trace.BadParameter("expected TOTP challenge")
}
