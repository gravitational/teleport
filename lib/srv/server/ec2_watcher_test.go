/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	liborganizations "github.com/gravitational/teleport/lib/utils/aws/organizations"
)

type mockEC2Client struct {
	output *ec2.DescribeInstancesOutput
	err    error
}

// Compile-time check that the mock satisfies the same API surface used by the
// EC2 fetcher tests.
var _ ec2.DescribeInstancesAPIClient = (*mockEC2Client)(nil)

func (m *mockEC2Client) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	// This mock intentionally applies AWS-side filtering semantics so the unit
	// tests validate fetcher filter behavior, not only happy-path wiring.
	var output ec2.DescribeInstancesOutput
	for _, res := range m.output.Reservations {
		var instances []ec2types.Instance
		for _, inst := range res.Instances {
			if instanceMatches(inst, input.Filters) {
				instances = append(instances, inst)
			}
		}
		output.Reservations = append(output.Reservations, ec2types.Reservation{
			Instances: instances,
		})
	}
	return &output, nil
}

type mockAWSAccountClient struct {
	output        *account.ListRegionsOutput
	responseError error
}

func (m *mockAWSAccountClient) ListRegions(ctx context.Context, input *account.ListRegionsInput, opts ...func(*account.Options)) (*account.ListRegionsOutput, error) {
	if m.responseError != nil {
		return nil, m.responseError
	}

	return m.output, nil
}

type mockOrganizationsClient struct {
	organizationID string
	rootOUID       string
	ouItems        map[string]ouItem
}

type ouItem struct {
	innerOUs               []string
	innerAccounts          []string
	innerNotActiveAccounts []string
}

func (m *mockOrganizationsClient) ListChildren(ctx context.Context, input *organizations.ListChildrenInput, opts ...func(*organizations.Options)) (*organizations.ListChildrenOutput, error) {
	if input.ChildType != organizationtypes.ChildTypeOrganizationalUnit {
		return nil, trace.NotImplemented("unexpected call to organizations.ListChildren, with ChildType != OU")
	}

	ouItem, ok := m.ouItems[*input.ParentId]
	if !ok {
		return nil, trace.NotFound("OU %s does not exist", *input.ParentId)
	}

	var children []organizationtypes.Child
	for _, ouID := range ouItem.innerOUs {
		children = append(children, organizationtypes.Child{
			Id:   aws.String(ouID),
			Type: organizationtypes.ChildTypeOrganizationalUnit,
		})
	}
	return &organizations.ListChildrenOutput{
		Children: children,
	}, nil
}

func (m *mockOrganizationsClient) ListRoots(ctx context.Context, input *organizations.ListRootsInput, opts ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
	rootARN := fmt.Sprintf("arn:aws:organizations::0000000000:root/%s/%s", m.organizationID, m.rootOUID)
	return &organizations.ListRootsOutput{
		Roots: []organizationtypes.Root{
			{
				Id:  aws.String(m.rootOUID),
				Arn: aws.String(rootARN),
			},
		},
	}, nil
}

func (m *mockOrganizationsClient) ListAccountsForParent(ctx context.Context, input *organizations.ListAccountsForParentInput, opts ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
	ouItem, ok := m.ouItems[*input.ParentId]
	if !ok {
		return nil, trace.NotFound("OU %s does not exist", *input.ParentId)
	}

	var accounts []organizationtypes.Account
	for _, accountID := range ouItem.innerAccounts {
		accountARN := fmt.Sprintf("arn:aws:organizations::0000000000:account/%s/%s", m.organizationID, accountID)
		accounts = append(accounts, organizationtypes.Account{
			Id:    aws.String(accountID),
			State: organizationtypes.AccountStateActive,
			Arn:   aws.String(accountARN),
		})
	}
	for _, accountID := range ouItem.innerNotActiveAccounts {
		accountARN := fmt.Sprintf("arn:aws:organizations::0000000000:account/%s/%s", m.organizationID, accountID)
		accounts = append(accounts, organizationtypes.Account{
			Id:    aws.String(accountID),
			State: organizationtypes.AccountStateSuspended,
			Arn:   aws.String(accountARN),
		})
	}
	return &organizations.ListAccountsForParentOutput{
		Accounts: accounts,
	}, nil
}

func instanceMatches(inst ec2types.Instance, filters []ec2types.Filter) bool {
	allMatched := true
	for _, filter := range filters {
		name := aws.ToString(filter.Name)
		val := filter.Values[0]
		if name == AWSInstanceStateName && inst.State.Name != ec2types.InstanceStateNameRunning {
			return false
		}
		for _, tag := range inst.Tags {
			if aws.ToString(tag.Key) != name[4:] {
				continue
			}
			allMatched = allMatched && aws.ToString(tag.Value) != val
		}
	}

	return !allMatched
}

func TestNewEC2InstanceFetcherTags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		config          ec2FetcherConfig
		expectedFilters []ec2types.Filter
	}{
		{
			name: "with glob key",
			config: ec2FetcherConfig{
				Matcher: types.AWSMatcher{
					Tags: types.Labels{
						"*":     []string{},
						"hello": []string{"other"},
					},
				},
			},
			expectedFilters: []ec2types.Filter{
				{
					Name:   aws.String(AWSInstanceStateName),
					Values: []string{string(ec2types.InstanceStateNameRunning)},
				},
			},
		},
		{
			name: "with no glob key",
			config: ec2FetcherConfig{
				Matcher: types.AWSMatcher{
					Tags: types.Labels{
						"hello": []string{"other"},
					},
				},
			},
			expectedFilters: []ec2types.Filter{
				{
					Name:   aws.String(AWSInstanceStateName),
					Values: []string{string(ec2types.InstanceStateNameRunning)},
				},
				{
					Name:   aws.String("tag:hello"),
					Values: []string{"other"},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetcher := newEC2InstanceFetcher(tc.config)
			require.Equal(t, tc.expectedFilters, fetcher.Filters)
		})
	}
}

func TestEC2Watcher(t *testing.T) {
	t.Parallel()
	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:   []string{"EC2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"teleport": {"yes"}},
			SSM:     &types.AWSSSM{},
		},
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:   []string{"EC2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"env": {"dev"}},
			SSM:     &types.AWSSSM{},
		},
		{
			Params:      &types.InstallerParams{},
			Types:       []string{"EC2"},
			Regions:     []string{"us-west-2"},
			Tags:        map[string]utils.Strings{"with-eice": {"please"}},
			Integration: "my-aws-integration",
			SSM:         &types.AWSSSM{},
		},
		{
			Params:  &types.InstallerParams{},
			Types:   []string{"EC2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"env": {"dev"}},
			SSM:     &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleARN: "alternate-role-arn",
			},
		},
		{
			Params:  &types.InstallerParams{},
			Types:   []string{"EC2"},
			Regions: []string{"*"},
			Tags:    map[string]utils.Strings{"teleport": {"yes"}},
			SSM:     &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleARN: "implicit-region",
			},
		},
	}

	present := ec2types.Instance{
		InstanceId: aws.String("instance-present"),
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("teleport"),
				Value: aws.String("yes"),
			},
			{
				Key:   aws.String("Name"),
				Value: aws.String("Present"),
			},
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}
	presentOther := ec2types.Instance{
		InstanceId: aws.String("instance-present-2"),
		Tags: []ec2types.Tag{{
			Key:   aws.String("env"),
			Value: aws.String("dev"),
		}},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}
	presentForEICE := ec2types.Instance{
		InstanceId: aws.String("instance-present-3"),
		Tags: []ec2types.Tag{{
			Key:   aws.String("with-eice"),
			Value: aws.String("please"),
		}},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}
	altAccountPresent := ec2types.Instance{
		InstanceId: aws.String("alternate-instance"),
		Tags: []ec2types.Tag{{
			Key:   aws.String("env"),
			Value: aws.String("dev"),
		}},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}

	instanceImplicitRegion := ec2types.Instance{
		InstanceId: aws.String("instance-implicit-region"),
		Tags: []ec2types.Tag{{
			Key:   aws.String("teleport"),
			Value: aws.String("yes"),
		}},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}

	ec2DescribeInstancesOutNoAssumeRole := ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{
				present,
				presentOther,
				presentForEICE,
				{
					InstanceId: aws.String("instance-absent"),
					Tags: []ec2types.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("prod"),
					}},
					State: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNameRunning,
					},
				},
				{
					InstanceId: aws.String("instance-absent-3"),
					Tags: []ec2types.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("prod"),
					}, {
						Key:   aws.String("teleport"),
						Value: aws.String("yes"),
					}},
					State: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNamePending,
					},
				},
			},
		}},
	}
	ec2DescribeInstancesOutAlternateAssumeRole := ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{
				altAccountPresent,
				{
					InstanceId: aws.String("alternate-absent"),
					Tags: []ec2types.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("prod"),
					}},
					State: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNameRunning,
					},
				},
			},
		}},
	}
	ec2DescribeInstancesOutOnlyImplicitRegions := ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{instanceImplicitRegion},
		}},
	}

	ec2ClientOutputsByRole := map[string]*ec2.DescribeInstancesOutput{
		"":                   &ec2DescribeInstancesOutNoAssumeRole,
		"alternate-role-arn": &ec2DescribeInstancesOutAlternateAssumeRole,
		"implicit-region":    &ec2DescribeInstancesOutOnlyImplicitRegions,
	}

	ec2ClientGetter := func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
		assumedRoles := awsconfig.AssumedRoles(opts...)
		var roleARN string

		for _, assumedRole := range assumedRoles {
			roleARN = assumedRole.RoleARN
		}

		return &mockEC2Client{
			output: ec2ClientOutputsByRole[roleARN],
		}, nil
	}

	regionsListerGetter := func(ctx context.Context, opts ...awsconfig.OptionsFn) (account.ListRegionsAPIClient, error) {
		return &mockAWSAccountClient{
			output: &account.ListRegionsOutput{
				Regions: []accounttypes.Region{
					{RegionName: aws.String("eu-south-1")},
					{RegionName: aws.String("eu-south-2")},
				},
			},
		}, nil
	}

	const noDiscoveryConfig = ""

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter:     ec2ClientGetter,
		RegionsListerGetter: regionsListerGetter,
		DiscoveryConfigName: noDiscoveryConfig,
	})
	require.NoError(t, err)

	watcher := NewWatcher[*EC2DiscoveryResult](t.Context())
	watcher.SetFetchers(noDiscoveryConfig, fetchers)

	go watcher.Run()

	expectedInstances := []*EC2Instances{
		{
			Region:     "us-west-2",
			Instances:  []EC2Instance{toEC2Instance(present)},
			Parameters: map[string]string{"token": "", "scriptName": ""},
		},
		{
			Region:     "us-west-2",
			Instances:  []EC2Instance{toEC2Instance(presentOther)},
			Parameters: map[string]string{"token": "", "scriptName": ""},
		},
		{
			Region:      "us-west-2",
			Instances:   []EC2Instance{toEC2Instance(presentForEICE)},
			Parameters:  map[string]string{"token": "", "scriptName": "", "sshdConfigPath": ""},
			Integration: "my-aws-integration",
		},
		{
			Region:        "us-west-2",
			Instances:     []EC2Instance{toEC2Instance(altAccountPresent)},
			Parameters:    map[string]string{"token": "", "scriptName": "", "sshdConfigPath": ""},
			AssumeRoleARN: "alternate-role-arn",
		},
		{
			Region:        "eu-south-1",
			Instances:     []EC2Instance{toEC2Instance(instanceImplicitRegion)},
			Parameters:    map[string]string{"token": "", "scriptName": "", "sshdConfigPath": ""},
			AssumeRoleARN: "implicit-region",
		},
		{
			Region:        "eu-south-2",
			Instances:     []EC2Instance{toEC2Instance(instanceImplicitRegion)},
			Parameters:    map[string]string{"token": "", "scriptName": "", "sshdConfigPath": ""},
			AssumeRoleARN: "implicit-region",
		},
	}

	// Collect all instances from all discovery results.
	// We have 5 fetchers (one per matcher), each producing one EC2DiscoveryResult.
	var actualInstances []*EC2Instances
	for range len(fetchers) {
		select {
		case result := <-watcher.InstancesC:
			actualInstances = append(actualInstances, result.Instances...)
		case <-t.Context().Done():
			require.Fail(t, "context canceled")
		}
	}

	require.ElementsMatch(t, expectedInstances, actualInstances)

	select {
	case result := <-watcher.InstancesC:
		require.Fail(t, "unexpected result: %v", result)
	default:
	}
}

func TestEC2WatcherWithMultipleAccounts(t *testing.T) {
	t.Parallel()
	organizationID := "o-abcdefghij"
	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:   []string{"ec2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"teleport": {"yes"}},
			SSM:     &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleName: "MyRole",
			},
			Organization: &types.AWSOrganizationMatcher{
				OrganizationID: organizationID,
				OrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
					Include: []string{types.Wildcard},
				},
			},
		},
	}

	instance01Account01 := ec2types.Instance{
		InstanceId: aws.String("instance01-account01"),
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("teleport"),
				Value: aws.String("yes"),
			},
			{
				Key:   aws.String("Name"),
				Value: aws.String("Present"),
			},
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}
	ec2DescribeInstancesAccount01 := &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{instance01Account01},
		}},
	}

	instance02Account02 := ec2types.Instance{
		InstanceId: aws.String("instance02-account02"),
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("teleport"),
				Value: aws.String("yes"),
			},
			{
				Key:   aws.String("Name"),
				Value: aws.String("Present"),
			},
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}
	ec2DescribeInstancesAccount02 := &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{instance02Account02},
		}},
	}

	ec2ClientOutputsByRoleARN := map[string]*ec2.DescribeInstancesOutput{
		"arn:aws:iam::000000000001:role/MyRole": ec2DescribeInstancesAccount01,
		"arn:aws:iam::000000000002:role/MyRole": ec2DescribeInstancesAccount02,
	}

	ec2ClientGetter := func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
		assumedRoles := awsconfig.AssumedRoles(opts...)
		var roleARN string

		for _, assumedRole := range assumedRoles {
			roleARN = assumedRole.RoleARN
		}

		return &mockEC2Client{
			output: ec2ClientOutputsByRoleARN[roleARN],
		}, nil
	}

	organizationsGetter := func(ctx context.Context, opts ...awsconfig.OptionsFn) (liborganizations.OrganizationsClient, error) {
		return &mockOrganizationsClient{
			organizationID: organizationID,
			rootOUID:       "r-123",
			ouItems: map[string]ouItem{
				"r-123": ouItem{
					innerOUs: []string{},
					innerAccounts: []string{
						"000000000001",
						"000000000002",
					},
				},
			},
		}, nil
	}

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter:        ec2ClientGetter,
		AWSOrganizationsGetter: organizationsGetter,
	})
	require.NoError(t, err)

	watcher := NewWatcher[*EC2DiscoveryResult](t.Context())
	watcher.SetFetchers("", fetchers)

	go watcher.Run()

	expectedInstances := []*EC2Instances{
		{
			Region:        "us-west-2",
			Instances:     []EC2Instance{toEC2Instance(instance01Account01)},
			Parameters:    map[string]string{"token": "", "scriptName": ""},
			AssumeRoleARN: "arn:aws:iam::000000000001:role/MyRole",
		},
		{
			Region:        "us-west-2",
			Instances:     []EC2Instance{toEC2Instance(instance02Account02)},
			Parameters:    map[string]string{"token": "", "scriptName": ""},
			AssumeRoleARN: "arn:aws:iam::000000000002:role/MyRole",
		},
	}

	// The organization fetcher returns a single EC2DiscoveryResult containing
	// instances from all accounts.
	select {
	case result := <-watcher.InstancesC:
		require.NotNil(t, result)
		require.ElementsMatch(t, expectedInstances, result.Instances)
	case <-t.Context().Done():
		require.Fail(t, "context canceled")
	}

	select {
	case result := <-watcher.InstancesC:
		require.Fail(t, "unexpected result: %v", result)
	default:
	}
}

func TestEC2WatcherWithMixedResults(t *testing.T) {
	t.Parallel()
	organizationID := "o-abcdefghij"
	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:   []string{"ec2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"teleport": {"yes"}},
			SSM:     &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleName: "MyRole",
			},
			Integration: "my-integration",
			Organization: &types.AWSOrganizationMatcher{
				OrganizationID: organizationID,
				OrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
					Include: []string{types.Wildcard},
				},
			},
		},
	}

	// Account 1: returns instances successfully
	instance01Account01 := ec2types.Instance{
		InstanceId: aws.String("instance01-account01"),
		Tags: []ec2types.Tag{
			{Key: aws.String("teleport"), Value: aws.String("yes")},
			{Key: aws.String("Name"), Value: aws.String("SuccessfulInstance1")},
		},
		State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
	}
	ec2DescribeInstancesAccount01 := &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{instance01Account01},
		}},
	}

	// Account 2: will return access denied error
	// Account 3: will also return access denied error (tests multiple errors)

	// Account 4: returns instances successfully (tests multiple successful accounts)
	instance01Account04 := ec2types.Instance{
		InstanceId: aws.String("instance01-account04"),
		Tags: []ec2types.Tag{
			{Key: aws.String("teleport"), Value: aws.String("yes")},
			{Key: aws.String("Name"), Value: aws.String("SuccessfulInstance4")},
		},
		State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
	}
	ec2DescribeInstancesAccount04 := &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{
			Instances: []ec2types.Instance{instance01Account04},
		}},
	}

	ec2ClientGetter := func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
		assumedRoles := awsconfig.AssumedRoles(opts...)
		var roleARN string
		for _, assumedRole := range assumedRoles {
			roleARN = assumedRole.RoleARN
		}

		switch roleARN {
		case "arn:aws:iam::000000000002:role/MyRole":
			// Account 2 fails with access denied
			return &mockEC2Client{
				err: trace.AccessDenied("ec2:DescribeInstances access denied for account 000000000002"),
			}, nil
		case "arn:aws:iam::000000000003:role/MyRole":
			// Account 3 also fails with access denied
			return &mockEC2Client{
				err: trace.AccessDenied("ec2:DescribeInstances access denied for account 000000000003"),
			}, nil
		case "arn:aws:iam::000000000004:role/MyRole":
			// Account 4 succeeds
			return &mockEC2Client{output: ec2DescribeInstancesAccount04}, nil
		default:
			// Account 1 succeeds
			return &mockEC2Client{output: ec2DescribeInstancesAccount01}, nil
		}
	}

	organizationsGetter := func(ctx context.Context, opts ...awsconfig.OptionsFn) (liborganizations.OrganizationsClient, error) {
		return &mockOrganizationsClient{
			organizationID: organizationID,
			rootOUID:       "r-123",
			ouItems: map[string]ouItem{
				"r-123": {
					innerOUs: []string{},
					innerAccounts: []string{
						"000000000001", // succeeds
						"000000000002", // fails - access denied
						"000000000003", // fails - access denied
						"000000000004", // succeeds
					},
				},
			},
		}, nil
	}

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter:        ec2ClientGetter,
		AWSOrganizationsGetter: organizationsGetter,
	})
	require.NoError(t, err)

	watcher := NewWatcher[*EC2DiscoveryResult](t.Context())
	watcher.SetFetchers("", fetchers)

	go watcher.Run()

	// Should receive a single result containing both instances and errors
	select {
	case result := <-watcher.InstancesC:
		require.NotNil(t, result)

		// Should have instances from accounts 1 and 4 (2 successful accounts)
		require.True(t, result.HasInstances(), "expected instances from successful accounts")
		require.Len(t, result.Instances, 2, "expected instances from 2 successful accounts")

		// Collect instance IDs for verification (order may vary)
		var instanceIDs []string
		for _, inst := range result.Instances {
			require.Equal(t, "us-west-2", inst.Region)
			for _, ec2Inst := range inst.Instances {
				instanceIDs = append(instanceIDs, ec2Inst.InstanceID)
			}
		}
		require.ElementsMatch(t, []string{"instance01-account01", "instance01-account04"}, instanceIDs)

		// Should have permission errors from accounts 2 and 3 (2 failed accounts)
		require.True(t, result.HasErrors(), "expected permission errors from failed accounts")
		require.Len(t, result.PermissionErrors, 2, "expected errors from 2 failed accounts")

		// Collect failed account IDs for verification (order may vary)
		var failedAccountIDs []string
		for _, permErr := range result.PermissionErrors {
			failedAccountIDs = append(failedAccountIDs, permErr.AccountID)
			require.Equal(t, "my-integration", permErr.Integration)
			require.Equal(t, "us-west-2", permErr.Region)
		}
		require.ElementsMatch(t, []string{"000000000002", "000000000003"}, failedAccountIDs)

	case <-t.Context().Done():
		require.Fail(t, "context canceled")
	}

	// No more results expected
	select {
	case result := <-watcher.InstancesC:
		require.Fail(t, "unexpected result: %v", result)
	default:
	}
}

func TestEC2WatcherWithOrganizationListRegionsAccessDeniedUsesPerAccountScope(t *testing.T) {
	t.Parallel()

	organizationID := "o-abcdefghij"
	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:       []string{"ec2"},
			Regions:     []string{types.Wildcard},
			Tags:        map[string]utils.Strings{"teleport": {"yes"}},
			SSM:         &types.AWSSSM{},
			Integration: "my-integration",
			AssumeRole: &types.AssumeRole{
				RoleName: "MyRole",
			},
			Organization: &types.AWSOrganizationMatcher{
				OrganizationID: organizationID,
				OrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
					Include: []string{types.Wildcard},
				},
			},
		},
	}

	organizationsGetter := func(ctx context.Context, opts ...awsconfig.OptionsFn) (liborganizations.OrganizationsClient, error) {
		return &mockOrganizationsClient{
			organizationID: organizationID,
			rootOUID:       "r-123",
			ouItems: map[string]ouItem{
				"r-123": {
					innerOUs: []string{},
					innerAccounts: []string{
						"000000000001",
						"000000000002",
					},
				},
			},
		}, nil
	}

	regionsListerGetter := func(ctx context.Context, opts ...awsconfig.OptionsFn) (account.ListRegionsAPIClient, error) {
		return &mockAWSAccountClient{
			responseError: trace.AccessDenied("User is not authorized to perform: account:ListRegions"),
		}, nil
	}

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter: func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
			return &mockEC2Client{output: &ec2.DescribeInstancesOutput{}}, nil
		},
		RegionsListerGetter:    regionsListerGetter,
		AWSOrganizationsGetter: organizationsGetter,
	})
	require.NoError(t, err)

	watcher := NewWatcher[*EC2DiscoveryResult](t.Context())
	watcher.SetFetchers("", fetchers)

	go watcher.Run()

	select {
	case result := <-watcher.InstancesC:
		require.NotNil(t, result)
		require.Empty(t, result.Instances)
		require.Len(t, result.PermissionErrors, 2)

		var gotAccountIDs []string
		for _, permErr := range result.PermissionErrors {
			require.Equal(t, usertasks.AutoDiscoverEC2IssuePermAccountDenied, permErr.IssueType)
			require.Equal(t, "my-integration", permErr.Integration)
			require.Empty(t, permErr.Region)
			gotAccountIDs = append(gotAccountIDs, permErr.AccountID)
		}

		require.ElementsMatch(t, []string{"000000000001", "000000000002"}, gotAccountIDs)
	case <-t.Context().Done():
		require.Fail(t, "context canceled")
	}

	select {
	case result := <-watcher.InstancesC:
		require.Fail(t, "unexpected result: %v", result)
	default:
	}
}

func TestEC2WatcherWithOrgAccessDenied(t *testing.T) {
	t.Parallel()

	organizationID := "o-abcdefghij"
	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:   []string{"ec2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"teleport": {"yes"}},
			SSM:     &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleName: "MyRole",
			},
			Integration: "my-integration",
			Organization: &types.AWSOrganizationMatcher{
				OrganizationID: organizationID,
				OrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
					Include: []string{types.Wildcard},
				},
			},
		},
	}

	organizationsGetter := func(ctx context.Context, opts ...awsconfig.OptionsFn) (liborganizations.OrganizationsClient, error) {
		return &mockAccessDeniedOrganizationsClient{}, nil
	}

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter: func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
			return &mockEC2Client{output: &ec2.DescribeInstancesOutput{}}, nil
		},
		AWSOrganizationsGetter: organizationsGetter,
	})
	require.NoError(t, err)

	watcher := NewWatcher[*EC2DiscoveryResult](t.Context())
	watcher.SetFetchers("", fetchers)

	go watcher.Run()

	select {
	case result := <-watcher.InstancesC:
		require.NotNil(t, result)
		require.Empty(t, result.Instances)
		require.Len(t, result.PermissionErrors, 1)
		require.Equal(t, usertasks.AutoDiscoverEC2IssuePermOrgDenied, result.PermissionErrors[0].IssueType)
		require.Equal(t, "my-integration", result.PermissionErrors[0].Integration)
	case <-t.Context().Done():
		require.Fail(t, "context canceled")
	}
}

// mockAccessDeniedOrganizationsClient returns AccessDenied from
// ListRoots, simulating an org-level permission failure.
type mockAccessDeniedOrganizationsClient struct{}

func (m *mockAccessDeniedOrganizationsClient) ListRoots(ctx context.Context, input *organizations.ListRootsInput, opts ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
	return nil, trace.AccessDenied("not authorized to perform: organizations:ListRoots")
}

func (m *mockAccessDeniedOrganizationsClient) ListChildren(ctx context.Context, input *organizations.ListChildrenInput, opts ...func(*organizations.Options)) (*organizations.ListChildrenOutput, error) {
	return nil, trace.AccessDenied("not authorized to perform: organizations:ListChildren")
}

func (m *mockAccessDeniedOrganizationsClient) ListAccountsForParent(ctx context.Context, input *organizations.ListAccountsForParentInput, opts ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
	return nil, trace.AccessDenied("not authorized to perform: organizations:ListAccountsForParent")
}

func TestMatchersToEC2InstanceFetchers(t *testing.T) {
	ec2ClientGetter := func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
		return nil, errors.New("ec2 client getter invocation must not fail when creating fetchers")
	}

	matchers := []types.AWSMatcher{{
		Params: &types.InstallerParams{
			InstallTeleport: true,
		},
		Types:   []string{"EC2"},
		Regions: []string{"us-west-2"},
		Tags:    map[string]utils.Strings{"*": {"*"}},
		SSM:     &types.AWSSSM{},
	}}

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers:        matchers,
		EC2ClientGetter: ec2ClientGetter,
	})
	require.NoError(t, err)
	require.NotEmpty(t, fetchers)
}

func TestConvertEC2InstancesToServerInfos(t *testing.T) {
	t.Parallel()
	expected, err := types.NewServerInfo(types.Metadata{
		Name: "aws-myaccount-myinstance",
	}, types.ServerInfoSpecV1{
		NewLabels: map[string]string{"aws/foo": "bar"},
	})
	require.NoError(t, err)

	ec2Instances := &EC2Instances{
		AccountID: "myaccount",
		Instances: []EC2Instance{
			{
				InstanceID: "myinstance",
				Tags:       map[string]string{"foo": "bar"},
			},
		},
	}
	serverInfos, err := ec2Instances.ServerInfos()
	require.NoError(t, err)
	require.Len(t, serverInfos, 1)

	require.Empty(t, cmp.Diff(expected, serverInfos[0]))
}

func TestMakeEvents(t *testing.T) {
	for _, tt := range []struct {
		name     string
		insts    *EC2Instances
		expected map[string]*usageeventsv1.ResourceCreateEvent
	}{
		{
			name: "script mode with teleport agents, returns node resource type",
			insts: &EC2Instances{
				EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				Instances: []EC2Instance{{
					InstanceID: "i-123456789012",
				}},
				DocumentName: "TeleportDiscoveryInstaller",
			},
			expected: map[string]*usageeventsv1.ResourceCreateEvent{
				"aws/i-123456789012": {
					ResourceType:   "node",
					ResourceOrigin: "cloud",
					CloudProvider:  "AWS",
				},
			},
		},
		{
			name: "script mode with openssh config, returns node.openssh resource type",
			insts: &EC2Instances{
				EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				Instances: []EC2Instance{{
					InstanceID: "i-123456789012",
				}},
				DocumentName: "TeleportAgentlessDiscoveryInstaller",
			},
			expected: map[string]*usageeventsv1.ResourceCreateEvent{
				"aws/i-123456789012": {
					ResourceType:   "node.openssh",
					ResourceOrigin: "cloud",
					CloudProvider:  "AWS",
				},
			},
		},
		{
			name: "eice mode, returns node.openssh-eice resource type",
			insts: &EC2Instances{
				EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE,
				Instances: []EC2Instance{{
					InstanceID: "i-123456789012",
				}},
			},
			expected: map[string]*usageeventsv1.ResourceCreateEvent{
				"aws/i-123456789012": {
					ResourceType:   "node.openssh-eice",
					ResourceOrigin: "cloud",
					CloudProvider:  "AWS",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.insts.MakeEvents()
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestToEC2Instances(t *testing.T) {
	sampleInstance := ec2types.Instance{
		InstanceId: aws.String("instance-001"),
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("teleport"),
				Value: aws.String("yes"),
			},
			{
				Key:   aws.String("Name"),
				Value: aws.String("MyInstanceName"),
			},
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}

	sampleInstanceWithoutName := ec2types.Instance{
		InstanceId: aws.String("instance-001"),
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("teleport"),
				Value: aws.String("yes"),
			},
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}

	for _, tt := range []struct {
		name     string
		input    []ec2types.Instance
		expected []EC2Instance
	}{
		{
			name:  "with name",
			input: []ec2types.Instance{sampleInstance},
			expected: []EC2Instance{{
				InstanceID: "instance-001",
				Tags: map[string]string{
					"Name":     "MyInstanceName",
					"teleport": "yes",
				},
				InstanceName:     "MyInstanceName",
				OriginalInstance: sampleInstance,
			}},
		},
		{
			name:  "without name",
			input: []ec2types.Instance{sampleInstanceWithoutName},
			expected: []EC2Instance{{
				InstanceID: "instance-001",
				Tags: map[string]string{
					"teleport": "yes",
				},
				OriginalInstance: sampleInstanceWithoutName,
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ToEC2Instances(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestAccountIDFromRoleARN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		roleARN       string
		wantAccountID string
		wantErr       require.ErrorAssertionFunc
	}{
		{
			name:          "empty ARN",
			roleARN:       "",
			wantAccountID: "",
			wantErr:       require.NoError,
		},
		{
			name:          "valid ARN",
			roleARN:       "arn:aws:iam::123456789012:role/teleport",
			wantAccountID: "123456789012",
			wantErr:       require.NoError,
		},
		{
			name:          "invalid ARN",
			roleARN:       "invalid-arn",
			wantAccountID: "",
			wantErr:       require.Error,
		},
		{
			name:          "ARN without account ID",
			roleARN:       "arn:aws:iam:::role/teleport",
			wantAccountID: "",
			wantErr:       require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accountID, err := accountIDFromRoleARN(tt.roleARN)
			tt.wantErr(t, err)
			require.Equal(t, tt.wantAccountID, accountID)
		})
	}
}

func TestSyntheticAccountIDIsUnambiguous(t *testing.T) {
	t.Parallel()

	base := syntheticAccountID("invalid-arn", "my-int", "us-west-2")
	require.NotEmpty(t, base)
	require.Contains(t, base, "unknown-account-")

	// Fallback account IDs are deterministic for the same scope.
	require.Equal(t, base, syntheticAccountID("invalid-arn", "my-int", "us-west-2"))

	// Field boundaries are preserved even when values contain delimiters.
	delimiterCollisionLeft := syntheticAccountID("scope-a|scope-b", "my-int", "")
	delimiterCollisionRight := syntheticAccountID("scope-a", "scope-b|my-int", "")
	require.NotEqual(t, delimiterCollisionLeft, delimiterCollisionRight)

	// Empty fields retain positional meaning.
	integrationOnlyScope := syntheticAccountID("", "my-int", "")
	roleOnlyScope := syntheticAccountID("my-int", "", "")
	require.NotEqual(t, integrationOnlyScope, roleOnlyScope)

	// Distinct scopes must remain distinct even when the same non-empty values
	// appear shifted between positions.
	integrationRegionScope := syntheticAccountID("", "my-int", "us-west-2")
	roleIntegrationScope := syntheticAccountID("my-int", "us-west-2", "")
	require.NotEqual(t, integrationRegionScope, roleIntegrationScope)
}

func TestAccountIDFromRoleARNOrWarn(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	ctx := context.Background()

	valid := accountIDFromRoleARNOrWarn(ctx, logger, "arn:aws:iam::123456789012:role/teleport", "my-int", "us-west-2")
	require.Equal(t, "123456789012", valid)

	invalid := accountIDFromRoleARNOrWarn(ctx, logger, "invalid-arn", "my-int", "us-west-2")
	require.NotEmpty(t, invalid)
	require.Contains(t, invalid, "unknown-account-")

	// Fallback account IDs are deterministic for the same scope.
	invalidAgain := accountIDFromRoleARNOrWarn(ctx, logger, "invalid-arn", "my-int", "us-west-2")
	require.Equal(t, invalid, invalidAgain)

	emptyRole := accountIDFromRoleARNOrWarn(ctx, logger, "", "my-int", "us-west-2")
	require.NotEmpty(t, emptyRole)
	require.Contains(t, emptyRole, "unknown-account-")

	noScope := accountIDFromRoleARNOrWarn(ctx, logger, "", "", "")
	require.NotEmpty(t, noScope)
	require.Contains(t, noScope, "unknown-account-")

	// Region remains part of fallback scoping when account ID is unknown.
	otherRegion := accountIDFromRoleARNOrWarn(ctx, logger, "", "my-int", "us-east-1")
	require.NotEqual(t, emptyRole, otherRegion)

	// Integration remains part of fallback scoping when account ID is unknown.
	otherIntegration := accountIDFromRoleARNOrWarn(ctx, logger, "", "other-int", "us-west-2")
	require.NotEqual(t, emptyRole, otherIntegration)

	// Field boundaries are preserved even when values contain delimiters.
	delimiterCollisionLeft := accountIDFromRoleARNOrWarn(ctx, logger, "scope-a|scope-b", "my-int", "")
	delimiterCollisionRight := accountIDFromRoleARNOrWarn(ctx, logger, "scope-a", "scope-b|my-int", "")
	require.NotEqual(t, delimiterCollisionLeft, delimiterCollisionRight)

	// Empty fields retain positional meaning.
	integrationOnlyScope := accountIDFromRoleARNOrWarn(ctx, logger, "", "my-int", "")
	roleOnlyScope := accountIDFromRoleARNOrWarn(ctx, logger, "my-int", "", "")
	require.NotEqual(t, integrationOnlyScope, roleOnlyScope)
}

func TestSSMRunCommandParameters(t *testing.T) {
	for _, tt := range []struct {
		name           string
		cfg            ec2FetcherConfig
		errCheck       require.ErrorAssertionFunc
		expectedParams map[string]string
	}{
		{
			name: "using custom ssm document",
			cfg: ec2FetcherConfig{
				Matcher: types.AWSMatcher{
					Params: &types.InstallerParams{
						InstallTeleport: true,
						JoinToken:       "my-token",
						ScriptName:      "default-installer",
					},
					SSM: &types.AWSSSM{
						DocumentName: "TeleportDiscoveryInstaller",
					},
				},
			},
			errCheck: require.NoError,
			expectedParams: map[string]string{
				"token":      "my-token",
				"scriptName": "default-installer",
			},
		},
		{
			name: "using custom ssm document without agentless install",
			cfg: ec2FetcherConfig{
				Matcher: types.AWSMatcher{
					Params: &types.InstallerParams{
						InstallTeleport: false,
						JoinToken:       "my-token",
						ScriptName:      "default-agentless-installer",
						SSHDConfig:      "/etc/ssh/sshd_config",
					},
					SSM: &types.AWSSSM{
						DocumentName: "TeleportDiscoveryInstaller",
					},
				},
			},
			errCheck: require.NoError,
			expectedParams: map[string]string{
				"token":          "my-token",
				"scriptName":     "default-agentless-installer",
				"sshdConfigPath": "/etc/ssh/sshd_config",
			},
		},
		{
			name: "using pre-defined AWS document",
			cfg: ec2FetcherConfig{
				Matcher: types.AWSMatcher{
					Params: &types.InstallerParams{
						InstallTeleport: true,
						JoinToken:       "my-token",
						ScriptName:      "default-installer",
					},
					SSM: &types.AWSSSM{
						DocumentName: "AWS-RunShellScript",
					},
				},
				ProxyPublicAddrGetter: func(ctx context.Context) (string, error) {
					return "proxy.example.com", nil
				},
			},
			errCheck: require.NoError,
			expectedParams: map[string]string{
				"commands": installerScriptChecksFor("proxy.example.com") + `bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com/v1/webapi/scripts/installer/default-installer | bash -s my-token"`,
			},
		},
		{
			name: "using pre-defined AWS document with env vars defined",
			cfg: ec2FetcherConfig{
				Matcher: types.AWSMatcher{
					Params: &types.InstallerParams{
						InstallTeleport: true,
						JoinToken:       "my-token",
						ScriptName:      "default-installer",
						Suffix:          "cluster-green",
					},
					SSM: &types.AWSSSM{
						DocumentName: "AWS-RunShellScript",
					},
				},
				ProxyPublicAddrGetter: func(ctx context.Context) (string, error) {
					return "proxy.example.com", nil
				},
			},
			errCheck: require.NoError,
			expectedParams: map[string]string{
				"commands": `export TELEPORT_INSTALL_SUFFIX=cluster-green; ` + installerScriptChecksFor("proxy.example.com") + `bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com/v1/webapi/scripts/installer/default-installer | bash -s my-token"`,
			},
		},
		{
			name: "error if using AWS-RunShellScript but proxy addr is not yet available",
			cfg: ec2FetcherConfig{
				Matcher: types.AWSMatcher{
					Params: &types.InstallerParams{
						InstallTeleport: true,
						JoinToken:       "my-token",
						ScriptName:      "default-installer",
						Suffix:          "cluster-green",
					},
					SSM: &types.AWSSSM{
						DocumentName: "AWS-RunShellScript",
					},
				},
				ProxyPublicAddrGetter: func(ctx context.Context) (string, error) {
					return "", trace.NotFound("proxy is not yet available")
				},
			},
			errCheck: require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ssmRunCommandParameters(t.Context(), tt.cfg)
			tt.errCheck(t, err)
			if tt.expectedParams != nil {
				require.Equal(t, tt.expectedParams, got)
			}
		})
	}
}

func TestEC2IAMPermissionError(t *testing.T) {
	t.Parallel()

	t.Run("with region and integration", func(t *testing.T) {
		underlyingErr := errors.New("access denied")
		permErr := &EC2IAMPermissionError{
			Integration:         "my-integration",
			AccountID:           "123456789012",
			Region:              "us-west-2",
			IssueType:           "ec2-perm-account-denied",
			DiscoveryConfigName: "my-config",
			Err:                 underlyingErr,
		}

		errMsg := permErr.Error()
		require.Contains(t, errMsg, "my-integration")
		require.Contains(t, errMsg, "123456789012")
		require.Contains(t, errMsg, "us-west-2")
		require.Contains(t, errMsg, "ec2-perm-account-denied")
		require.Contains(t, errMsg, "access denied")
	})

	t.Run("without region", func(t *testing.T) {
		underlyingErr := errors.New("org access denied")
		permErr := &EC2IAMPermissionError{
			Integration: "my-integration",
			AccountID:   "123456789012",
			IssueType:   "ec2-perm-org-denied",
			Err:         underlyingErr,
		}

		errMsg := permErr.Error()
		require.Contains(t, errMsg, "my-integration")
		require.Contains(t, errMsg, "123456789012")
		require.Contains(t, errMsg, "ec2-perm-org-denied")
		require.NotContains(t, errMsg, "region")
	})

	t.Run("unwrap", func(t *testing.T) {
		underlyingErr := errors.New("access denied")
		permErr := &EC2IAMPermissionError{Err: underlyingErr}

		require.ErrorIs(t, permErr, underlyingErr)
	})

	t.Run("without integration", func(t *testing.T) {
		permErr := &EC2IAMPermissionError{
			AccountID: "123456789012",
			Region:    "us-west-2",
			IssueType: "ec2-perm-account-denied",
			Err:       errors.New("access denied"),
		}

		errMsg := permErr.Error()
		require.NotContains(t, errMsg, "integration")
		require.Contains(t, errMsg, "123456789012")
		require.Contains(t, errMsg, "us-west-2")
	})

	t.Run("errors.As through trace.Wrap", func(t *testing.T) {
		underlyingErr := errors.New("access denied")
		permErr := &EC2IAMPermissionError{
			AccountID: "123456789012",
			IssueType: "ec2-perm-account-denied",
			Err:       underlyingErr,
		}

		wrappedErr := trace.Wrap(permErr)

		var found *EC2IAMPermissionError
		require.ErrorAs(t, wrappedErr, &found)
		require.Equal(t, "123456789012", found.AccountID)
	})
}

func TestEC2DiscoveryResultCollectError(t *testing.T) {
	t.Parallel()

	t.Run("direct permission error", func(t *testing.T) {
		result := &EC2DiscoveryResult{}
		permErr := &EC2IAMPermissionError{
			AccountID: "123456789012",
			IssueType: "ec2-perm-account-denied",
			Err:       errors.New("access denied"),
		}

		collected := result.collectError(permErr)
		require.True(t, collected)
		require.Len(t, result.PermissionErrors, 1)
		require.Equal(t, "123456789012", result.PermissionErrors[0].AccountID)
	})

	t.Run("wrapped permission error", func(t *testing.T) {
		result := &EC2DiscoveryResult{}
		permErr := &EC2IAMPermissionError{
			AccountID: "123456789012",
			IssueType: "ec2-perm-account-denied",
			Err:       errors.New("access denied"),
		}
		wrappedErr := trace.Wrap(permErr)

		collected := result.collectError(wrappedErr)
		require.True(t, collected)
		require.Len(t, result.PermissionErrors, 1)
	})

	t.Run("non-permission error ignored", func(t *testing.T) {
		result := &EC2DiscoveryResult{}
		regularErr := errors.New("some other error")

		collected := result.collectError(regularErr)
		require.False(t, collected)
		require.Empty(t, result.PermissionErrors)
	})
}

