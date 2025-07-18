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
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
)

func TestListRolesAnywhereProfiles(t *testing.T) {
	exampleProfile := ratypes.ProfileDetail{
		Name:                  aws.String("ExampleProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid1"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
	}

	syncProfile := ratypes.ProfileDetail{
		Name:                  aws.String("SyncProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid2"),
		Enabled:               aws.Bool(true),
		AcceptRoleSessionName: aws.Bool(true),
	}

	disabledProfile := ratypes.ProfileDetail{
		Name:                  aws.String("SyncProfile"),
		ProfileArn:            aws.String("arn:aws:rolesanywhere:eu-west-2:123456789012:profile/uuid3"),
		Enabled:               aws.Bool(false),
		AcceptRoleSessionName: aws.Bool(true),
	}

	client := &mockListRolesAnywhereProfiles{
		profiles: []ratypes.ProfileDetail{
			exampleProfile,
			syncProfile,
			disabledProfile,
		},
	}
	resp, err := ListRolesAnywhereProfiles(t.Context(), client, ListRolesAnywhereProfilesRequest{})
	require.NoError(t, err)

	require.Len(t, resp.Profiles, 3)
}

func TestListRolesAnywhereProfilesPage(t *testing.T) {
	baseRequestWithMockedProfiles := func(profiles map[string]ratypes.ProfileDetail) func() listRolesAnywhereProfilesRequest {
		return func() listRolesAnywhereProfilesRequest {
			return listRolesAnywhereProfilesRequest{
				raClient: &mockIAMRolesAnywhere{
					profilesByID: profiles,
				},
			}
		}
	}
	rolesAnywhereProfileWithName := func(name string) ratypes.ProfileDetail {
		return ratypes.ProfileDetail{
			Name:       aws.String(name),
			ProfileArn: aws.String(uuid.NewString()),
		}
	}

	for _, tt := range []struct {
		name             string
		req              func() listRolesAnywhereProfilesRequest
		expectedResp     func(t *testing.T, page []*integrationv1.RolesAnywhereProfile)
		expectedErrCheck require.ErrorAssertionFunc
	}{
		{
			name: "no resources",
			req:  baseRequestWithMockedProfiles(nil),
			expectedResp: func(t *testing.T, page []*integrationv1.RolesAnywhereProfile) {
				require.Empty(t, page)
			},
			expectedErrCheck: require.NoError,
		},
		{
			name: "some resources",
			req: baseRequestWithMockedProfiles(map[string]ratypes.ProfileDetail{
				"id1": rolesAnywhereProfileWithName("ExampleProfile"),
			}),
			expectedResp: func(t *testing.T, page []*integrationv1.RolesAnywhereProfile) {
				require.Len(t, page, 1)
				require.Equal(t, "ExampleProfile", page[0].Name)
			},
			expectedErrCheck: require.NoError,
		},
		{
			name: "with filters using glob",
			req: func() listRolesAnywhereProfilesRequest {
				baseReq := baseRequestWithMockedProfiles(map[string]ratypes.ProfileDetail{
					"id1": rolesAnywhereProfileWithName("TeamA-subteam1"),
					"id2": rolesAnywhereProfileWithName("TeamA-subteam2"),
					"id3": rolesAnywhereProfileWithName("TeamA-subteam3"),

					"id4": rolesAnywhereProfileWithName("TeamB-subteam1"),
					"id5": rolesAnywhereProfileWithName("TeamB-subteam2"),
					"id6": rolesAnywhereProfileWithName("TeamB-subteam3"),
				})()
				baseReq.filters = []string{"TeamB-*"}
				return baseReq
			},
			expectedResp: func(t *testing.T, page []*integrationv1.RolesAnywhereProfile) {
				require.Len(t, page, 3)
				profile := page[0]
				require.NotEmpty(t, profile.Arn)
				require.Contains(t, profile.Name, "TeamB-subteam")
			},
			expectedErrCheck: require.NoError,
		},
		{
			name: "with filters using regex",
			req: func() listRolesAnywhereProfilesRequest {
				baseReq := baseRequestWithMockedProfiles(map[string]ratypes.ProfileDetail{
					"id1": rolesAnywhereProfileWithName("TeamA-subteam1"),
					"id2": rolesAnywhereProfileWithName("TeamA-subteam2"),
					"id3": rolesAnywhereProfileWithName("TeamA-subteam3"),

					"id4": rolesAnywhereProfileWithName("TeamB-subteam1"),
					"id5": rolesAnywhereProfileWithName("TeamB-subteam2"),
					"id6": rolesAnywhereProfileWithName("TeamB-subteam3"),

					"id7": rolesAnywhereProfileWithName("TeamC-subteam1"),
					"id8": rolesAnywhereProfileWithName("TeamC-subteam2"),
					"id9": rolesAnywhereProfileWithName("TeamC-subteam3"),

					"id10": rolesAnywhereProfileWithName("AnotherTeam"),
				})()
				baseReq.filters = []string{`^Team[A,B]{1}\-subteam\d$`}
				return baseReq
			},
			expectedResp: func(t *testing.T, page []*integrationv1.RolesAnywhereProfile) {
				require.Len(t, page, 6)
				profileNames := make([]string, 0, len(page))
				for _, profile := range page {
					profileNames = append(profileNames, profile.Name)
				}
				require.Contains(t, profileNames, "TeamA-subteam1")
			},
			expectedErrCheck: require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, _, err := listRolesAnywhereProfilesPage(t.Context(), tt.req())
			tt.expectedErrCheck(t, err)
			tt.expectedResp(t, resp)
		})
	}
}

type mockListRolesAnywhereProfiles struct {
	profiles []ratypes.ProfileDetail
}

func (m *mockListRolesAnywhereProfiles) ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error) {
	return &rolesanywhere.ListProfilesOutput{
		Profiles:  m.profiles,
		NextToken: nil,
	}, nil
}

func (m *mockListRolesAnywhereProfiles) ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error) {
	return &rolesanywhere.ListTagsForResourceOutput{
		Tags: []ratypes.Tag{
			{Key: aws.String("MyTagKey"), Value: aws.String("my-tag-value")},
		},
	}, nil
}
