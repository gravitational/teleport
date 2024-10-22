/*
Copyright 2024 Gravitational, Inc.

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
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
)

func TestPerformMFACeremony(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testMFAChallenge := &proto.MFAAuthenticateChallenge{
		TOTP: &proto.TOTPChallenge{},
	}
	testMFAResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: "otp-test-code",
			},
		},
	}

	for _, tt := range []struct {
		name                   string
		ceremony               *mfa.Ceremony
		assertCeremonyResponse func(*testing.T, *proto.MFAAuthenticateResponse, error, ...interface{})
	}{
		{
			name: "OK ceremony success prompt",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return testMFAChallenge, nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return testMFAResponse, nil
					})
				},
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.NoError(t, err)
				assert.Equal(t, testMFAResponse, mr)
			},
		}, {
			name: "OK ceremony not required",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return &proto.MFAAuthenticateChallenge{
						MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
					}, nil
				},
				PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return nil, trace.BadParameter("expected mfa not required")
					})
				},
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.Error(t, err, mfa.ErrMFANotRequired)
				assert.Nil(t, mr)
			},
		}, {
			name: "NOK create challenge fail",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return nil, errors.New("create authenticate challenge failure")
				},
				PromptConstructor: func(opts ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return nil, trace.BadParameter("expected challenge failure")
					})
				},
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "create authenticate challenge failure")
				assert.Nil(t, mr)
			},
		}, {
			name: "NOK prompt mfa fail",
			ceremony: &mfa.Ceremony{
				CreateAuthenticateChallenge: func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
					return testMFAChallenge, nil
				},
				PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
					return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
						return nil, errors.New("prompt mfa failure")
					})
				},
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "prompt mfa failure")
				assert.Nil(t, mr)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.ceremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
				MFARequiredCheck: &proto.IsMFARequiredRequest{},
			})
			tt.assertCeremonyResponse(t, resp, err)
		})
	}
}
