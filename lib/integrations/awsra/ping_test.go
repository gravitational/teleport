/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	exampleProfile := ratypes.ProfileDetail{
		Name:                  aws.String("ExampleProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
		RoleArns: []string{
			"arn:aws:iam::123456789012:role/ExampleRole",
			"arn:aws:iam::123456789012:role/SyncRole",
		},
	}

	syncProfile := ratypes.ProfileDetail{
		Name:                  aws.String("SyncProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid2"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
		RoleArns: []string{
			"arn:aws:iam::123456789012:role/ExampleRole",
			"arn:aws:iam::123456789012:role/SyncRole",
		},
	}

	disabledProfile := ratypes.ProfileDetail{
		Name:                  aws.String("SyncProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid3"),
		Enabled:               aws.Bool(false),
		AcceptRoleSessionName: aws.Bool(true),
	}

	profileWithoutRoles := ratypes.ProfileDetail{
		Name:                  aws.String("SyncProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid4"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
	}

	t.Run("ping returns the caller identity and 2 enabled profiles", func(t *testing.T) {
		resp, err := Ping(t.Context(), &mockPingClient{
			accountID: "123456789012",
			profiles: []ratypes.ProfileDetail{
				exampleProfile,
				syncProfile,
				disabledProfile,
				profileWithoutRoles,
			},
		}, *syncProfile.ProfileArn)
		require.NoError(t, err)

		require.Equal(t, "123456789012", resp.AccountID)
		require.Equal(t, 1, resp.EnabledProfileCounter)
	})

}

type mockPingClient struct {
	accountID string
	profiles  []ratypes.ProfileDetail
}

func (m *mockPingClient) ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error) {
	return &rolesanywhere.ListProfilesOutput{
		Profiles:  m.profiles,
		NextToken: nil,
	}, nil
}

func (m *mockPingClient) ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error) {
	return &rolesanywhere.ListTagsForResourceOutput{
		Tags: []ratypes.Tag{
			{Key: aws.String("MyTagKey"), Value: aws.String("my-tag-value")},
		},
	}, nil
}

func (m *mockPingClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String(m.accountID),
		Arn:     aws.String("arn:aws:iam::123456789012:user/test-user"),
		UserId:  aws.String("USERID1234567890"),
	}, nil
}
