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
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// PingResponse contains the identity information and how many Roles Anywhere Profiles are active and have at least one Role.
type PingResponse struct {
	// AccountID is the AWS account ID of the caller.
	AccountID string
	// ARN is the ARN of the caller.
	ARN string
	// UserID is the user ID of the caller.
	UserID string
	// EnabledProfileCounter is the number of Roles Anywhere Profiles.
	// Disabled profiles, profiles without assigned roles and the ProfileSync profile are not counted.
	EnabledProfileCounter int
}

// PingClient describes the required methods to list AWS VPCs.
type PingClient interface {
	RolesAnywhereProfilesLister
	CallerIdentityGetter
}

type defaultPingClient struct {
	RolesAnywhereProfilesLister
	stsClient CallerIdentityGetter
}

// GetCallerIdentity returns information about the caller identity.
func (c *defaultPingClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return c.stsClient.GetCallerIdentity(ctx, params, optFns...)
}

// NewPingClient creates a new PingClient using an AWSClientCredentials.
func NewPingClient(ctx context.Context, req *AWSClientConfig) (PingClient, error) {
	awsConfig, err := newAWSConfig(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultPingClient{
		RolesAnywhereProfilesLister: rolesanywhere.NewFromConfig(awsConfig),
		stsClient:                   stsutils.NewFromConfig(awsConfig),
	}, nil
}

// Ping calls the following AWS API:
// https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_ListProfiles.html
// https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_ListTagsForResource.html
// It returns a list of Roles Anywhere Profiles that are enabled.
//
// It will ignore any profile matching ignoredProfileARN.
func Ping(ctx context.Context, clt PingClient, ignoredProfileARNs []string) (*PingResponse, error) {
	var errs []error

	profileCounter := 0
	var nextToken *string
	for {
		listReq := listRolesAnywhereProfilesRequest{
			nextPage:           nextToken,
			ignoredProfileARNs: ignoredProfileARNs,
		}
		profiles, nextPageToken, err := listRolesAnywhereProfilesPage(ctx, clt, listReq)
		if err != nil {
			errs = append(errs, err)
			break
		}
		for _, profile := range profiles {
			// Ignore disabled profiles and profiles without assigned roles.
			if profile.Enabled && len(profile.Roles) > 0 {
				profileCounter++
			}
		}
		if aws.ToString(nextPageToken) == "" {
			break
		}
		nextToken = nextPageToken
	}

	callerIdentity, err := clt.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, trace.NewAggregate(errs...)
	}

	return &PingResponse{
		EnabledProfileCounter: profileCounter,
		AccountID:             aws.ToString(callerIdentity.Account),
		ARN:                   aws.ToString(callerIdentity.Arn),
		UserID:                aws.ToString(callerIdentity.UserId),
	}, nil
}
