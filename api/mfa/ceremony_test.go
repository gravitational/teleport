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

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
)

func TestPerformMFACeremony(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testMFAResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: "otp-test-code",
			},
		},
	}

	for _, tt := range []struct {
		name                   string
		ceremonyClient         *fakeMFACeremonyClient
		assertCeremonyResponse func(*testing.T, *proto.MFAAuthenticateResponse, error, ...interface{})
	}{
		{
			name: "OK ceremony success",
			ceremonyClient: &fakeMFACeremonyClient{
				challengeResponse: testMFAResponse,
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.NoError(t, err)
				assert.Equal(t, testMFAResponse, mr)
			},
		}, {
			name: "OK ceremony not required",
			ceremonyClient: &fakeMFACeremonyClient{
				challengeResponse: testMFAResponse,
				mfaRequired:       proto.MFARequired_MFA_REQUIRED_NO,
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.Error(t, err, mfa.ErrMFANotRequired)
				assert.Nil(t, mr)
			},
		}, {
			name: "NOK create challenge fail",
			ceremonyClient: &fakeMFACeremonyClient{
				challengeResponse:              testMFAResponse,
				createAuthenticateChallengeErr: errors.New("create authenticate challenge failure"),
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "create authenticate challenge failure")
				assert.Nil(t, mr)
			},
		}, {
			name: "NOK prompt mfa fail",
			ceremonyClient: &fakeMFACeremonyClient{
				challengeResponse: testMFAResponse,
				promptMFAErr:      errors.New("prompt mfa failure"),
			},
			assertCeremonyResponse: func(t *testing.T, mr *proto.MFAAuthenticateResponse, err error, i ...interface{}) {
				assert.ErrorContains(t, err, "prompt mfa failure")
				assert.Nil(t, mr)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := mfa.PerformMFACeremony(ctx, tt.ceremonyClient, &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
				MFARequiredCheck: &proto.IsMFARequiredRequest{},
			})
			tt.assertCeremonyResponse(t, resp, err)
		})
	}
}

type fakeMFACeremonyClient struct {
	createAuthenticateChallengeErr error
	promptMFAErr                   error
	mfaRequired                    proto.MFARequired
	challengeResponse              *proto.MFAAuthenticateResponse
}

func (c *fakeMFACeremonyClient) CreateAuthenticateChallenge(ctx context.Context, in *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	if c.createAuthenticateChallengeErr != nil {
		return nil, c.createAuthenticateChallengeErr
	}

	chal := &proto.MFAAuthenticateChallenge{
		TOTP: &proto.TOTPChallenge{},
	}

	if in.MFARequiredCheck != nil {
		chal.MFARequired = c.mfaRequired
	}

	return chal, nil
}

func (c *fakeMFACeremonyClient) PromptMFA(ctx context.Context, chal *proto.MFAAuthenticateChallenge, promptOpts ...mfa.PromptOpt) (*proto.MFAAuthenticateResponse, error) {
	if c.promptMFAErr != nil {
		return nil, c.promptMFAErr
	}

	return c.challengeResponse, nil
}
