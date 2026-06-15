/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package aws_sync

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/require"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/mocks"
)

// mockEC2Client implements ec2.DescribeInstancesAPIClient for tests.
type mockEC2Client struct {
	reservations []ec2types.Reservation
}

func (m *mockEC2Client) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{Reservations: m.reservations}, nil
}

// iamInstanceProfilesMock embeds mocks.IAMMock (satisfying the full iamClient
// interface) and overrides ListInstanceProfiles with controlled behavior.
type iamInstanceProfilesMock struct {
	mocks.IAMMock
	profiles []iamtypes.InstanceProfile
	err      error
}

func (m *iamInstanceProfilesMock) ListInstanceProfiles(_ context.Context, _ *iam.ListInstanceProfilesInput, _ ...func(*iam.Options)) (*iam.ListInstanceProfilesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &iam.ListInstanceProfilesOutput{InstanceProfiles: m.profiles}, nil
}

// TestFetchAWSEC2InstancesMultipleRegions verifies that instances from all regions
// are collected correctly and that concurrent region fetching is free of data races.
// Run with -race to detect the pre-fix race where goroutines appended directly to
// the shared hosts slice without the protecting mutex.
func TestFetchAWSEC2InstancesMultipleRegions(t *testing.T) {
	const accountID = "123456789012"

	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}

	// Each region gets two distinct instances.
	instancesByRegion := map[string][]ec2types.Instance{
		"us-east-1": {
			{InstanceId: aws.String("i-us-east-1a"), PublicDnsName: aws.String("a.us-east-1.compute.amazonaws.com")},
			{InstanceId: aws.String("i-us-east-1b"), PublicDnsName: aws.String("b.us-east-1.compute.amazonaws.com")},
		},
		"us-west-2": {
			{InstanceId: aws.String("i-us-west-2a"), PublicDnsName: aws.String("a.us-west-2.compute.amazonaws.com")},
			{InstanceId: aws.String("i-us-west-2b"), PublicDnsName: aws.String("b.us-west-2.compute.amazonaws.com")},
		},
		"eu-west-1": {
			{InstanceId: aws.String("i-eu-west-1a"), PublicDnsName: aws.String("a.eu-west-1.compute.amazonaws.com")},
			{InstanceId: aws.String("i-eu-west-1b"), PublicDnsName: aws.String("b.eu-west-1.compute.amazonaws.com")},
		},
	}

	getEC2Client := func(_ context.Context, region string, _ ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
		return &mockEC2Client{
			reservations: []ec2types.Reservation{
				{Instances: instancesByRegion[region]},
			},
		}, nil
	}

	a := &Fetcher{
		Config: Config{
			AccountID:    accountID,
			Regions:      regions,
			GetEC2Client: getEC2Client,
		},
		lastResult: &Resources{},
	}

	instances, err := a.fetchAWSEC2Instances(t.Context())
	require.NoError(t, err)
	// 3 regions × 2 instances each = 6 total.
	require.Len(t, instances, 6)

	// Verify each region's instances are present with correct region/account metadata.
	byID := make(map[string]*accessgraphv1alpha.AWSInstanceV1, len(instances))
	for _, inst := range instances {
		byID[inst.InstanceId] = inst
	}
	for region, regionInstances := range instancesByRegion {
		for _, ec2inst := range regionInstances {
			id := aws.ToString(ec2inst.InstanceId)
			got, ok := byID[id]
			require.True(t, ok, "expected instance %s to be present", id)
			require.Equal(t, region, got.Region)
			require.Equal(t, accountID, got.AccountId)
		}
	}
}

// TestFetchAWSEC2InstancesClientError verifies that when the EC2 client returns an
// error for a region, the instances from the previous sync for that region are
// returned as a fallback.
func TestFetchAWSEC2InstancesClientError(t *testing.T) {
	const accountID = "123456789012"

	regions := []string{"us-east-1", "us-west-2"}

	existingInstances := []*accessgraphv1alpha.AWSInstanceV1{
		{InstanceId: "i-existing", Region: "us-east-1", AccountId: accountID},
	}

	getEC2Client := func(_ context.Context, region string, _ ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
		if region == "us-east-1" {
			return nil, errors.New("unauthorized")
		}
		return &mockEC2Client{
			reservations: []ec2types.Reservation{
				{Instances: []ec2types.Instance{
					{InstanceId: aws.String("i-us-west-2a")},
				}},
			},
		}, nil
	}

	a := &Fetcher{
		Config: Config{
			AccountID:    accountID,
			Regions:      regions,
			GetEC2Client: getEC2Client,
		},
		lastResult: &Resources{
			Instances: existingInstances,
		},
	}

	instances, err := a.fetchAWSEC2Instances(t.Context())
	// Error is collected but does not prevent partial results.
	require.Error(t, err)
	// The failed region falls back to existing instances; the successful region
	// contributes its new instance.
	require.Len(t, instances, 2)

	byID := make(map[string]*accessgraphv1alpha.AWSInstanceV1, len(instances))
	for _, inst := range instances {
		byID[inst.InstanceId] = inst
	}
	require.Contains(t, byID, "i-existing")
	require.Contains(t, byID, "i-us-west-2a")
}

// TestFetchInstanceProfilesErrorReturnsExisting verifies that on a pager error the
// function returns the previous sync's profiles unchanged, not a mix of partial
// new results and old results (pre-fix behavior was append(profiles, existing...)).
func TestFetchInstanceProfilesErrorReturnsExisting(t *testing.T) {
	const accountID = "123456789012"

	existingProfiles := []*accessgraphv1alpha.AWSInstanceProfileV1{
		{InstanceProfileId: "profile-existing", AccountId: accountID},
	}

	a := &Fetcher{
		Config: Config{
			AccountID:         accountID,
			Regions:           []string{"us-east-1"},
			AWSConfigProvider: &mocks.AWSConfigProvider{},
			awsClients: fakeAWSClients{
				iamClient: &iamInstanceProfilesMock{
					err: errors.New("access denied"),
				},
			},
		},
		lastResult: &Resources{
			InstanceProfiles: existingProfiles,
		},
	}

	profiles, err := a.fetchInstanceProfiles(t.Context())
	require.Error(t, err)
	// Must equal existing exactly — no partial new data mixed in.
	require.Equal(t, existingProfiles, profiles)
}

// TestFetchInstanceProfilesSuccess verifies that all profiles are returned on a
// successful fetch.
func TestFetchInstanceProfilesSuccess(t *testing.T) {
	const accountID = "123456789012"

	iamProfiles := []iamtypes.InstanceProfile{
		{
			InstanceProfileId:   aws.String("AIPA1"),
			InstanceProfileName: aws.String("profile1"),
			Arn:                 aws.String("arn:aws:iam::123456789012:instance-profile/profile1"),
			Path:                aws.String("/"),
		},
		{
			InstanceProfileId:   aws.String("AIPA2"),
			InstanceProfileName: aws.String("profile2"),
			Arn:                 aws.String("arn:aws:iam::123456789012:instance-profile/profile2"),
			Path:                aws.String("/"),
		},
	}

	a := &Fetcher{
		Config: Config{
			AccountID:         accountID,
			Regions:           []string{"us-east-1"},
			AWSConfigProvider: &mocks.AWSConfigProvider{},
			awsClients: fakeAWSClients{
				iamClient: &iamInstanceProfilesMock{profiles: iamProfiles},
			},
		},
		lastResult: &Resources{},
	}

	profiles, err := a.fetchInstanceProfiles(t.Context())
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	byID := make(map[string]*accessgraphv1alpha.AWSInstanceProfileV1, len(profiles))
	for _, p := range profiles {
		byID[p.InstanceProfileId] = p
	}
	require.Contains(t, byID, "AIPA1")
	require.Contains(t, byID, "AIPA2")
}
