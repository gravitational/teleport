// Copyright 2022 Gravitational, Inc
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

package gcp

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/googleapis/gax-go/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testIAMCredentialsClient struct {
	generateAccessToken func(ctx context.Context, req *credentialspb.GenerateAccessTokenRequest, opts ...gax.CallOption) (*credentialspb.GenerateAccessTokenResponse, error)
}

func (i *testIAMCredentialsClient) GenerateAccessToken(ctx context.Context, req *credentialspb.GenerateAccessTokenRequest, opts ...gax.CallOption) (*credentialspb.GenerateAccessTokenResponse, error) {
	return i.generateAccessToken(ctx, req, opts...)
}

var _ iamCredentialsClient = (*testIAMCredentialsClient)(nil)

func makeTestCloudClient(client *testIAMCredentialsClient) cloudClientGCP {
	return &cloudClientGCPImpl[*testIAMCredentialsClient]{getGCPIAMClient: func(ctx context.Context) (*testIAMCredentialsClient, error) {
		return client, nil
	}}
}

func TestHandler_getToken(t *testing.T) {
	mkConstConfig := func(val HandlerConfig) func(any) HandlerConfig {
		return func(_ any) HandlerConfig {
			return val
		}
	}

	tests := []struct {
		name string

		initState func() any

		config func(state any) HandlerConfig

		wantToken  *credentialspb.GenerateAccessTokenResponse
		checkErr   require.ErrorAssertionFunc
		checkState func(require.TestingT, any)
	}{
		{
			name: "base case",
			config: mkConstConfig(HandlerConfig{
				cloudClientGCP: makeTestCloudClient(&testIAMCredentialsClient{
					generateAccessToken: func(ctx context.Context, req *credentialspb.GenerateAccessTokenRequest, opts ...gax.CallOption) (*credentialspb.GenerateAccessTokenResponse, error) {
						if req.GetName() != "projects/-/serviceAccounts/MY_ACCOUNT" {
							return nil, trace.BadParameter("wrong serviceAccount, expected %q got %q", "projects/-/serviceAccounts/MY_ACCOUNT", req.GetName())
						}
						if !assert.ObjectsAreEqual(req.GetScope(), defaultScopeList) {
							return nil, trace.BadParameter("wrong scopes")
						}
						return &credentialspb.GenerateAccessTokenResponse{AccessToken: "ok"}, nil
					},
				}),
			}),
			wantToken: &credentialspb.GenerateAccessTokenResponse{AccessToken: "ok"},
			checkErr:  require.NoError,
		},
		{
			name: "timeout",
			initState: func() any {
				return clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 12, 00, 00, 000, time.UTC))
			},
			config: func(state any) HandlerConfig {
				return HandlerConfig{
					Clock: state.(clockwork.FakeClock).(clockwork.Clock),
					cloudClientGCP: makeTestCloudClient(&testIAMCredentialsClient{
						generateAccessToken: func(ctx context.Context, req *credentialspb.GenerateAccessTokenRequest, opts ...gax.CallOption) (*credentialspb.GenerateAccessTokenResponse, error) {
							clock := state.(clockwork.FakeClock)

							// advance time by getTokenTimeout
							clock.Advance(getTokenTimeout)

							// after the test is done unblock the sleep() below.
							t.Cleanup(func() {
								clock.Advance(getTokenTimeout * 2)
							})

							// block for 2*getTokenTimeout; this call won't return before Cleanup() phase.
							clock.Sleep(getTokenTimeout * 2)

							return nil, trace.BadParameter("bad param foo")
						},
					}),
				}
			},
			checkErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "timeout waiting for access token for 5s")
				require.ErrorIs(t, err, context.DeadlineExceeded)
			},
		},
		{
			name: "non-timeout error",
			config: mkConstConfig(HandlerConfig{
				cloudClientGCP: makeTestCloudClient(&testIAMCredentialsClient{
					generateAccessToken: func(ctx context.Context, req *credentialspb.GenerateAccessTokenRequest, opts ...gax.CallOption) (*credentialspb.GenerateAccessTokenResponse, error) {
						return nil, trace.BadParameter("bad param foo")
					},
				}),
			}),
			checkErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bad param foo")
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var state any
			if tt.initState != nil {
				state = tt.initState()
			}

			ctx := context.Background()

			fwd, err := newGCPHandler(ctx, tt.config(state))
			require.NoError(t, err)

			token, err := fwd.getToken(ctx, "MY_ACCOUNT")
			require.Equal(t, tt.wantToken, token)
			tt.checkErr(t, err)

			if tt.checkState != nil {
				tt.checkState(t, state)
			}
		})
	}
}

func TestHandler_getToken_cache(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()

	calls := 0
	fwd, err := newGCPHandler(ctx, HandlerConfig{
		Clock: clock,
		cloudClientGCP: makeTestCloudClient(&testIAMCredentialsClient{
			generateAccessToken: func(ctx context.Context, req *credentialspb.GenerateAccessTokenRequest, opts ...gax.CallOption) (*credentialspb.GenerateAccessTokenResponse, error) {
				calls++
				return &credentialspb.GenerateAccessTokenResponse{AccessToken: "ok"}, nil
			},
		}),
	})
	require.NoError(t, err)

	// first call goes through
	_, err = fwd.getToken(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 1, calls)

	// second call is cached
	_, err = fwd.getToken(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 1, calls)

	// advance past cache expiry
	clock.Advance(time.Second * 60 * 2)

	// third call goes through
	_, err = fwd.getToken(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
