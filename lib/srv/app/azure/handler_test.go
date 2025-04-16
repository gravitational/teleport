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

	testCaseContext, testCaseContextCancel := context.WithCancel(context.Background())

	type testCase struct {
		name string

		getTokenContext context.Context
		config          HandlerConfig

		managedIdentity string
		scope           string

		wantToken *azcore.AccessToken
		checkErr  require.ErrorAssertionFunc
	}

	var tests []testCase

	tests = []testCase{
		{
			name:            "base case",
			getTokenContext: context.Background(),
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
			name:            "timeout",
			getTokenContext: context.Background(),
			config: HandlerConfig{
				Clock: clockwork.NewFakeClock(),
				getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
					// find the fake clock from above
					var clock *clockwork.FakeClock
					for _, test := range tests {
						if test.name == "timeout" {
							clock = test.config.Clock.(*clockwork.FakeClock)
						}
					}

					// advance time by getTokenTimeout
					clock.Advance(getTokenTimeout)

					// after the test is done unblock the sleep() below.
					t.Cleanup(func() {
						clock.Advance(getTokenTimeout * 2)
					})

					// block for 2*getTokenTimeout; this call won't return before Cleanup() phase.
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
			name:            "non-timeout error",
			getTokenContext: context.Background(),
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
		{
			name:            "context cancel",
			getTokenContext: testCaseContext,
			config: HandlerConfig{
				getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
					testCaseContextCancel()
					return nil, trace.BadParameter("bad param foo")
				},
			},
			checkErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, context.Canceled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hnd, err := newAzureHandler(context.Background(), tt.config)
			require.NoError(t, err)

			token, err := hnd.getToken(tt.getTokenContext, tt.managedIdentity, tt.scope)

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
