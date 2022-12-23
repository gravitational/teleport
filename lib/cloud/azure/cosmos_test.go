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
	"github.com/stretchr/testify/require"
)

func TestGetKey(t *testing.T) {
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
			cl := NewCosmosDatabaseAccountsClientByAPI(tc.client)
			res, err := cl.GetKey(ctx, "resource-group", "database-account", tc.key)
			tc.expectErr(t, err)
			require.Equal(t, tc.expectedValue, res)
		})
	}
}

func TestRegenerateKey(t *testing.T) {
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
			caseCtx, caseCtxCancel := context.WithCancel(ctx)
			defer caseCtxCancel()

			doneCh := make(chan error)
			cl := NewCosmosDatabaseAccountsClientByAPI(tc.client)

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
