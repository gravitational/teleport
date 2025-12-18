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
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	"github.com/gravitational/trace"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// ListRolesAnywhereProfilesRequest contains the required fields to list the Roles Anywhere Profiles in Amazon IAM.
type ListRolesAnywhereProfilesRequest struct {
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
	// PageSize is the maximum number of records to retrieve.
	PageSize int
	// ProfileNameFilters is a list of filters applied to the profile name.
	// Only matching profiles will be synchronized as application servers.
	// If empty, no filtering is applied.
	//
	// Filters can be globs, for example:
	//
	//	profile*
	//	*name*
	//
	// Or regexes if they're prefixed and suffixed with ^ and $, for example:
	//
	//	^profile.*$
	//	^.*name.*$
	ProfileNameFilters []string
	// IgnoredProfileARNs is a list of profile ARNs that should be ignored.
	IgnoredProfileARNs []string
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
	listReq := listRolesAnywhereProfilesRequest{
		nextPage:           nextToken,
		pageSize:           req.PageSize,
		filters:            req.ProfileNameFilters,
		ignoredProfileARNs: req.IgnoredProfileARNs,
	}
	profiles, nextPageToken, err := listRolesAnywhereProfilesPage(ctx, clt, listReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ListRolesAnywhereProfilesResponse{
		Profiles:  profiles,
		NextToken: aws.ToString(nextPageToken),
	}, nil
}

type listRolesAnywhereProfilesRequest struct {
	nextPage           *string
	pageSize           int
	filters            []string
	ignoredProfileARNs []string
}

func listRolesAnywhereProfilesPage(ctx context.Context, raClient RolesAnywhereProfilesLister, req listRolesAnywhereProfilesRequest) (ret []*integrationv1.RolesAnywhereProfile, nextToken *string, err error) {
	var pageSizeRequest *int32
	if req.pageSize > 0 {
		pageSizeRequest = aws.Int32(int32(req.pageSize))
	}

	nextPage := req.nextPage

	for {
		profilesListResp, err := raClient.ListProfiles(ctx, &rolesanywhere.ListProfilesInput{
			NextToken: nextPage,
			PageSize:  pageSizeRequest,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		profilesAsProto, err := convertRolesAnywhereProfilePageToProto(ctx, profilesListResp, raClient, req)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		ret = append(ret, profilesAsProto...)

		// If no profile was added (because of filters), and there's more resources, fetch the next page.
		// This ensures that the client doesn't receive an empty page when there are more resources to fetch.
		nextPage = profilesListResp.NextToken
		if len(ret) == 0 && aws.ToString(nextPage) != "" {
			continue
		}
		break
	}

	return ret, nextPage, nil
}

func convertRolesAnywhereProfilePageToProto(ctx context.Context, profilesListResp *rolesanywhere.ListProfilesOutput, raClient RolesAnywhereProfilesLister, req listRolesAnywhereProfilesRequest) ([]*integrationv1.RolesAnywhereProfile, error) {
	var ret []*integrationv1.RolesAnywhereProfile

	for _, profile := range profilesListResp.Profiles {
		if slices.Contains(req.ignoredProfileARNs, aws.ToString(profile.ProfileArn)) {
			continue
		}

		matches, err := profileNameMatchesFilters(aws.ToString(profile.Name), req.filters)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !matches {
			continue
		}

		profileTags, err := raClient.ListTagsForResource(ctx, &rolesanywhere.ListTagsForResourceInput{
			ResourceArn: profile.ProfileArn,
		})
		if err != nil {
			return nil, trace.Wrap(err)
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

	return ret, nil
}

func profileNameMatchesFilters(profileName string, filters []string) (bool, error) {
	// If no filters are provided, all profiles match.
	if len(filters) == 0 {
		return true, nil
	}

	for _, filter := range filters {
		matches, err := utils.MatchString(profileName, filter)
		if err != nil {
			return false, trace.Wrap(err, "error parsing filter: %s", filter)
		}
		if matches {
			return true, nil
		}
	}
	return false, nil
}
