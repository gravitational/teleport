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
	"errors"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestCosmosGetKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, tc := range []struct {
		desc          string
		client        armCosmosDatabaseAccountsClient
		key           armcosmos.KeyKind
		expectedValue string
		expectErr     require.ErrorAssertionFunc
	}{
		{
			desc: "PrimaryMasterKey",
			client: &ARMCosmosDatabaseAccountsMock{
				PrimaryMasterKey: "primary-key",
			},
			key:           armcosmos.KeyKindPrimary,
			expectedValue: "primary-key",
			expectErr:     require.NoError,
		},
		{
			desc: "PrimaryReadonlyKey",
			client: &ARMCosmosDatabaseAccountsMock{
				PrimaryReadOnlyKey: "primary-readonly-key",
			},
			key:           armcosmos.KeyKindPrimaryReadonly,
			expectedValue: "primary-readonly-key",
			expectErr:     require.NoError,
		},
		{
			desc: "SecondaryMasterKey",
			client: &ARMCosmosDatabaseAccountsMock{
				SecondaryMasterKey: "secondary-key",
			},
			key:           armcosmos.KeyKindSecondary,
			expectedValue: "secondary-key",
			expectErr:     require.NoError,
		},
		{
			desc: "SecondaryReadonlyKey",
			client: &ARMCosmosDatabaseAccountsMock{
				SecondaryReadOnlyKey: "secondary-readonly-key",
			},
			key:           armcosmos.KeyKindSecondaryReadonly,
			expectedValue: "secondary-readonly-key",
			expectErr:     require.NoError,
		},
		{
			desc: "AuthenticationError",
			client: &ARMCosmosDatabaseAccountsMock{
				NoAuth: true,
			},
			key:       armcosmos.KeyKindPrimary,
			expectErr: require.Error,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fncache, err := newDefaultCosmosFnCache()
			require.NoError(t, err)
			cl := NewCosmosDatabaseAccountsClientByAPI(tc.client, fncache)
			res, err := cl.GetKey(ctx, "resource-group", "database-account", tc.key)
			tc.expectErr(t, err)
			require.Equal(t, tc.expectedValue, res)
		})
	}
}

func TestCosmosGetKeyCache(t *testing.T) {
	ctx := context.Background()
	ttl := 10 * time.Second

	setupCosmosDBClient := func(t *testing.T) (CosmosDatabaseAccountsClient, *ARMCosmosDatabaseAccountsMock, clockwork.FakeClock) {
		clock := clockwork.NewFakeClock()
		fncache, err := utils.NewFnCache(utils.FnCacheConfig{
			TTL:     ttl,
			Context: ctx,
			Clock:   clock,
		})
		require.NoError(t, err)

		mock := &ARMCosmosDatabaseAccountsMock{PrimaryMasterKey: "firstkeyvalue"}
		return NewCosmosDatabaseAccountsClientByAPI(mock, fncache), mock, clock
	}

	t.Run("return cached key", func(t *testing.T) {
		cl, mock, clock := setupCosmosDBClient(t)

		// Fetch it the first time, no cache is present, so it should return the
		// information from mock.
		firstKey, err := cl.GetKey(ctx, "", "", armcosmos.KeyKindPrimary)
		require.NoError(t, err)
		require.Equal(t, "firstkeyvalue", firstKey)

		// Update the mock and issue a second GetKey request. This time it should
		// return the cached result.
		mock.PrimaryMasterKey = "secondkeyvalue"
		secondKey, err := cl.GetKey(ctx, "", "", armcosmos.KeyKindPrimary)
		require.NoError(t, err)
		require.Equal(t, "firstkeyvalue", secondKey)

		// Advance in time to make the cache expire and issue another request.
		// Now it should return the latest value from mock.
		clock.Advance(ttl + 1)
		thirdKey, err := cl.GetKey(ctx, "", "", armcosmos.KeyKindPrimary)
		require.NoError(t, err)
		require.Equal(t, "secondkeyvalue", thirdKey)
	})

	t.Run("return Azure error", func(t *testing.T) {
		cl, mock, clock := setupCosmosDBClient(t)

		// Fetch it the first time, no error, so return the contents from mock.
		firstKey, err := cl.GetKey(ctx, "", "", armcosmos.KeyKindPrimary)
		require.NoError(t, err)
		require.Equal(t, "firstkeyvalue", firstKey)

		// Update mock to return error but since there is cache, it should
		// return no error.
		mock.NoAuth = true
		secondKey, err := cl.GetKey(ctx, "", "", armcosmos.KeyKindPrimary)
		require.NoError(t, err)
		require.Equal(t, "firstkeyvalue", secondKey)

		// Advance in time to make the cache expire and issue another request.
		// Now it should return error.
		clock.Advance(ttl + 1)
		_, err = cl.GetKey(ctx, "", "", armcosmos.KeyKindPrimary)
		require.Error(t, err)
	})
}

func TestCosmosRegenerateKey(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range []struct {
		desc      string
		client    armCosmosDatabaseAccountsClient
		key       armcosmos.KeyKind
		expectErr require.ErrorAssertionFunc
	}{
		{
			desc: "RegeneratesKey",
			client: &ARMCosmosDatabaseAccountsMock{
				RegenerateKeyDone: true,
			},
			expectErr: require.NoError,
		},
		{
			desc:   "RegeneratesKeyDeadline",
			client: &ARMCosmosDatabaseAccountsMock{},
			expectErr: func(t require.TestingT, err error, _ ...interface{}) {
				unwrapped := trace.Unwrap(err)
				if errors.Is(context.DeadlineExceeded, unwrapped) || errors.Is(context.Canceled, unwrapped) {
					return
				}

				require.Fail(t, "unexpected error type", "expected context error but got %T", unwrapped)
			},
		},
		{
			desc: "AuthenticationError",
			client: &ARMCosmosDatabaseAccountsMock{
				NoAuth: true,
			},
			expectErr: func(t require.TestingT, err error, _ ...interface{}) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %T", err)
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fncache, err := newDefaultCosmosFnCache()
			require.NoError(t, err)

			caseCtx, caseCtxCancel := context.WithCancel(ctx)
			defer caseCtxCancel()

			doneCh := make(chan error)
			cl := NewCosmosDatabaseAccountsClientByAPI(tc.client, fncache)

			go func() {
				doneCh <- cl.RegenerateKey(caseCtx, "resource-group", "database-account", tc.key)
			}()

			select {
			case err := <-doneCh:
				tc.expectErr(t, err)
			default:
			}

			// If the doneCh didn't send anything, cancel the regenerate key and
			// assert its result.
			caseCtxCancel()
			tc.expectErr(t, <-doneCh)
		})
	}
}
