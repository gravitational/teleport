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
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwarder_getToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		config HandlerConfig

		serviceAccount string

		wantToken *credentialspb.GenerateAccessTokenResponse
		checkErr  require.ErrorAssertionFunc
	}{
		{
			name: "base case",
			config: HandlerConfig{
				generateAccessToken: func(ctx context.Context, serviceAccount string, scopes []string) (*credentialspb.GenerateAccessTokenResponse, error) {
					if serviceAccount != "MY_ACCOUNT" {
						return nil, trace.BadParameter("wrong serviceAccount, expected %q got %q", "MY_ACCOUNT", serviceAccount)
					}
					if !assert.ObjectsAreEqual(scopes, defaultScopeList) {
						return nil, trace.BadParameter("wrong scopes")
					}

					return &credentialspb.GenerateAccessTokenResponse{AccessToken: "ok"}, nil
				},
			},
			serviceAccount: "MY_ACCOUNT",
			wantToken:      &credentialspb.GenerateAccessTokenResponse{AccessToken: "ok"},
			checkErr:       require.NoError,
		},
		{
			name: "timeout",
			config: HandlerConfig{
				generateAccessToken: func(ctx context.Context, serviceAccount string, scopes []string) (*credentialspb.GenerateAccessTokenResponse, error) {
					time.Sleep(getTokenTimeout * 2)
					return nil, trace.BadParameter("some error")
				},
			},
			checkErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "timeout waiting for access token for 5s")
				require.ErrorIs(t, err, context.DeadlineExceeded)
			},
		},
		{
			name: "non-timeout error",
			config: HandlerConfig{
				generateAccessToken: func(ctx context.Context, serviceAccount string, scopes []string) (*credentialspb.GenerateAccessTokenResponse, error) {
					return nil, trace.BadParameter("bad param foo")
				},
			},
			checkErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bad param foo")
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			fwd, err := newGCPHandler(ctx, tt.config)
			require.NoError(t, err)

			token, err := fwd.getToken(ctx, tt.serviceAccount)
			tt.checkErr(t, err)
			require.Equal(t, tt.wantToken, token)
		})
	}
}

func TestForwarder_getToken_cache(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()

	calls := 0
	fwd, err := newGCPHandler(ctx, HandlerConfig{
		Clock: clock,
		generateAccessToken: func(ctx context.Context, serviceAccount string, scopes []string) (*credentialspb.GenerateAccessTokenResponse, error) {
			calls++
			return &credentialspb.GenerateAccessTokenResponse{AccessToken: "ok"}, nil
		},
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
