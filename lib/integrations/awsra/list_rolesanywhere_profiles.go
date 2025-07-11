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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	"github.com/gravitational/trace"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
)

// ListRolesAnywhereProfilesRequest contains the required fields to list the Roles Anywhere Profiles in Amazon IAM.
type ListRolesAnywhereProfilesRequest struct {
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
	// PageSize is the maximum number of records to retrieve.
	PageSize int
}

// ListRolesAnywhereProfilesResponse contains a page of Roles Anywhere Profiles.
type ListRolesAnywhereProfilesResponse struct {
	// Profiles contains the page of Roles Anywhere Profiles.
	Profiles []*integrationv1.RolesAnywhereProfile `json:"profiles"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken"`
}

// RolesAnywhereProfilesLister is an interface that defines methods to interact with the AWS IAM Roles Anywhere service.
type RolesAnywhereProfilesLister interface {
	// Lists all profiles in the authenticated account and Amazon Web Services Region.
	ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error)

	// Lists the tags attached to the resource.
	ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error)
}

type defaultListRolesAnywhereProfilesClient struct {
	RolesAnywhereProfilesLister
}

// NewListRolesAnywhereProfilesClient creates a new ListRolesAnywhereProfilesClient using an AWSClientRequest.
func NewListRolesAnywhereProfilesClient(ctx context.Context, req *AWSClientConfig) (RolesAnywhereProfilesLister, error) {
	awsConfig, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultListRolesAnywhereProfilesClient{
		RolesAnywhereProfilesLister: rolesanywhere.NewFromConfig(awsConfig),
	}, nil
}

// ListRolesAnywhereProfiles calls the following AWS API:
// https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_ListProfiles.html
// https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_ListTagsForResource.html
// It returns a list of Roles Anywhere Profiles and an optional NextToken that can be used to fetch the next page.
func ListRolesAnywhereProfiles(ctx context.Context, clt RolesAnywhereProfilesLister, req ListRolesAnywhereProfilesRequest) (*ListRolesAnywhereProfilesResponse, error) {
	var nextToken *string
	if req.NextToken != "" {
		nextToken = aws.String(req.NextToken)
	}
	profiles, nextPageToken, err := listRolesAnywhereProfilesPage(ctx, clt, nextToken, req.PageSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ListRolesAnywhereProfilesResponse{
		Profiles:  profiles,
		NextToken: aws.ToString(nextPageToken),
	}, nil
}

func listRolesAnywhereProfilesPage(ctx context.Context, raClient RolesAnywhereProfilesLister, nextPage *string, pageSize int) ([]*integrationv1.RolesAnywhereProfile, *string, error) {
	var ret []*integrationv1.RolesAnywhereProfile

	var pageSizeRequest *int32
	if pageSize > 0 {
		pageSizeRequest = aws.Int32(int32(pageSize))
	}

	profilesListResp, err := raClient.ListProfiles(ctx, &rolesanywhere.ListProfilesInput{
		NextToken: nextPage,
		PageSize:  pageSizeRequest,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	for _, profile := range profilesListResp.Profiles {
		profileTags, err := raClient.ListTagsForResource(ctx, &rolesanywhere.ListTagsForResourceInput{
			ResourceArn: profile.ProfileArn,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		labels := make(map[string]string, len(profileTags.Tags))
		for _, tag := range profileTags.Tags {
			labels[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
		}

		ret = append(ret, &integrationv1.RolesAnywhereProfile{
			Arn:                   aws.ToString(profile.ProfileArn),
			Name:                  aws.ToString(profile.Name),
			Enabled:               aws.ToBool(profile.Enabled),
			AcceptRoleSessionName: aws.ToBool(profile.AcceptRoleSessionName),
			Roles:                 profile.RoleArns,
			Tags:                  labels,
		})
	}

	return ret, profilesListResp.NextToken, nil
}
