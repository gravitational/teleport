/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package awsoidc

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCheckAndSetDefaults(t *testing.T) {
	t.Run("invalid regions must return an error", func(t *testing.T) {
		err := (&AWSClientRequest{
			Token:   "token",
			RoleARN: "some-arn",
			Region:  "?",
		}).CheckAndSetDefaults()

		require.True(t, trace.IsBadParameter(err))
	})
	t.Run("valid region", func(t *testing.T) {
		err := (&AWSClientRequest{
			Token:   "token",
			RoleARN: "some-arn",
			Region:  "us-east-1",
		}).CheckAndSetDefaults()
		require.NoError(t, err)
	})

	t.Run("empty region", func(t *testing.T) {
		err := (&AWSClientRequest{
			Token:   "token",
			RoleARN: "some-arn",
			Region:  "",
		}).CheckAndSetDefaults()
		require.NoError(t, err)
	})
}

func TestCheckAccountID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc            string
		clt             CallerIdentityGetter
		accountID       string
		wantErrContains string
	}{
		{
			desc:      "caller identity matches expected account ID",
			clt:       mockSTSClient{accountID: "123456789012"},
			accountID: "123456789012",
		},
		{
			desc: "empty expected account ID is always valid",
			clt:  mockSTSClient{accountID: "123456789012"},
		},
		{
			desc:            "caller identity does not match expected account ID",
			clt:             mockSTSClient{accountID: "123456789012"},
			accountID:       "222222222222",
			wantErrContains: "expected account ID 222222222222 but current account ID is 123456789012",
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := CheckAccountID(ctx, test.clt, test.accountID)
			if test.wantErrContains != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.NoError(t, err)
		})
	}
}
