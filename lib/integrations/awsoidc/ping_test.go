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

	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"
)

var _ PingClient = (*mockPingClient)(nil)

type mockPingClient struct {
	accountID string
	arn       string
	userID    string
}

// Returns the ping information
func (m mockPingClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: &m.accountID,
		Arn:     &m.arn,
		UserId:  &m.userID,
	}, nil
}

func TestPing(t *testing.T) {
	ctx := context.Background()

	pingResp, err := Ping(ctx, mockPingClient{
		accountID: "123",
		arn:       "arn:123",
		userID:    "U123",
	})
	require.NoError(t, err)
	require.Equal(t, "123", pingResp.AccountID)
	require.Equal(t, "arn:123", pingResp.ARN)
	require.Equal(t, "U123", pingResp.UserID)
}
