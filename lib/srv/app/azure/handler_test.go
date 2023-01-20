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

package azure

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestForwarder_getToken(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string

		config HandlerConfig

		managedIdentity string
		scope           string

		wantToken *azcore.AccessToken
		checkErr  require.ErrorAssertionFunc
	}

	var tests []testCase

	tests = []testCase{
		{
			name: "base case",
			config: HandlerConfig{
				getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
					if managedIdentity != "MY_IDENTITY" {
						return nil, trace.BadParameter("wrong managedIdentity")
					}
					if scope != "MY_SCOPE" {
						return nil, trace.BadParameter("wrong scope")
					}
					return &azcore.AccessToken{Token: "foobar"}, nil
				},
			},
			managedIdentity: "MY_IDENTITY",
			scope:           "MY_SCOPE",
			wantToken:       &azcore.AccessToken{Token: "foobar"},
			checkErr:        require.NoError,
		},
		{
			name: "timeout",
			config: HandlerConfig{
				Clock: clockwork.NewFakeClock(),
				getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
					// find the fake clock from above
					var clock clockwork.FakeClock
					for _, test := range tests {
						if test.name == "timeout" {
							clock = test.config.Clock.(clockwork.FakeClock)
						}
					}

					clock.Advance(getTokenTimeout)
					clock.Sleep(getTokenTimeout * 2)
					return &azcore.AccessToken{Token: "foobar"}, nil
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
				getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
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

			hnd, err := newAzureHandler(ctx, tt.config)
			require.NoError(t, err)

			token, err := hnd.getToken(ctx, tt.managedIdentity, tt.scope)

			require.Equal(t, tt.wantToken, token)
			tt.checkErr(t, err)
		})
	}
}

func TestForwarder_getToken_cache(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()

	calls := 0
	hnd, err := newAzureHandler(ctx, HandlerConfig{
		Clock: clock,
		getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
			calls++
			return &azcore.AccessToken{Token: "OK"}, nil
		},
	})
	require.NoError(t, err)

	// first call goes through
	_, err = hnd.getToken(ctx, "", "")
	require.NoError(t, err)
	require.Equal(t, 1, calls)

	// second call is cached
	_, err = hnd.getToken(ctx, "", "")
	require.NoError(t, err)
	require.Equal(t, 1, calls)

	// advance past cache expiry
	clock.Advance(time.Second * 60 * 2)

	// third call goes through
	_, err = hnd.getToken(ctx, "", "")
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
