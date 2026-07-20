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
	"testing"
	"testing/synctest"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	liborganizations "github.com/gravitational/teleport/lib/utils/aws/organizations"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type mockEC2Client struct {
	output        *ec2.DescribeInstancesOutput
	responseError error
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.responseError != nil {
		return nil, m.responseError
	}

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
			OwnerId:   res.OwnerId,
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
	responseError  error
}

type ouItem struct {
	innerOUs               []string
	innerAccounts          []string
	innerNotActiveAccounts []string
}

func (m *mockOrganizationsClient) ListChildren(ctx context.Context, input *organizations.ListChildrenInput, opts ...func(*organizations.Options)) (*organizations.ListChildrenOutput, error) {
	if m.responseError != nil {
		return nil, m.responseError
	}
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
	if m.responseError != nil {
		return nil, m.responseError
	}
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
	if m.responseError != nil {
		return nil, m.responseError
	}
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

	watcher := NewWatcher[*EC2Instances](t.Context(), logtest.NewLogger())
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

	var gotInstances []*EC2Instances
	for len(gotInstances) < len(expectedInstances) {
		select {
		case instances := <-watcher.InstancesC:
			require.NotNil(t, instances)
			gotInstances = append(gotInstances, instances)
		case <-t.Context().Done():
			require.Fail(t, "context canceled")
		}
	}
	require.ElementsMatch(t, expectedInstances, gotInstances)

	select {
	case result := <-watcher.InstancesC:
		require.Fail(t, "unexpected result: %v", result)
	default:
	}
}

func TestEC2WatcherMergesReservationInstances(t *testing.T) {
	matchers := []types.AWSMatcher{{
		Params: &types.InstallerParams{
			InstallTeleport: true,
		},
		Types:   []string{"ec2"},
		Regions: []string{"us-west-2"},
		Tags:    map[string]utils.Strings{"teleport": {"yes"}},
		SSM:     &types.AWSSSM{},
	}}

	present := ec2types.Instance{
		InstanceId: aws.String("instance-present"),
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String("Present"),
			},
			{
				Key:   aws.String("teleport"),
				Value: aws.String("yes"),
			},
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}
	presentOther := ec2types.Instance{
		InstanceId: aws.String("instance-present-2"),
		Tags: []ec2types.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String("PresentOther"),
			},
			{
				Key:   aws.String("teleport"),
				Value: aws.String("yes"),
			},
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}

	const noDiscoveryConfig = ""

	t.Run("same account id in multiple reservations are merged", func(t *testing.T) {
		ec2DescribeInstancesOneOwnerReservation := ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{Instances: []ec2types.Instance{present}, OwnerId: aws.String("123456789012")},
				{Instances: []ec2types.Instance{presentOther}, OwnerId: aws.String("123456789012")},
			},
		}

		ec2ClientGetter := func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
			return &mockEC2Client{
				output: &ec2DescribeInstancesOneOwnerReservation,
			}, nil
		}

		fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
			Matchers: matchers,
			PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
				return "proxy.example.com:3080", nil
			},
			EC2ClientGetter:     ec2ClientGetter,
			DiscoveryConfigName: noDiscoveryConfig,
		})
		require.NoError(t, err)

		watcher := NewWatcher[*EC2Instances](t.Context(), logtest.NewLogger())
		watcher.SetFetchers(noDiscoveryConfig, fetchers)

		go watcher.Run()

		expectedInstances := &EC2Instances{
			Region:     "us-west-2",
			Instances:  []EC2Instance{toEC2Instance(present), toEC2Instance(presentOther)},
			Parameters: map[string]string{"token": "", "scriptName": ""},
			AccountID:  "123456789012",
		}

		select {
		case instances := <-watcher.InstancesC:
			require.NotNil(t, instances)
			require.Equal(t, expectedInstances, instances)
		case <-t.Context().Done():
			require.Fail(t, "context canceled")
		}
	})

	t.Run("if owner is not the same, they are not merged", func(t *testing.T) {
		ec2DescribeInstancesOneInstancePerReservation := ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{Instances: []ec2types.Instance{present}, OwnerId: aws.String("123456789012")},
				{Instances: []ec2types.Instance{presentOther}, OwnerId: aws.String("210987654321")},
			},
		}

		ec2ClientGetter := func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
			return &mockEC2Client{
				output: &ec2DescribeInstancesOneInstancePerReservation,
			}, nil
		}

		fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
			Matchers: matchers,
			PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
				return "proxy.example.com:3080", nil
			},
			EC2ClientGetter:     ec2ClientGetter,
			DiscoveryConfigName: noDiscoveryConfig,
		})
		require.NoError(t, err)

		var gotInstances []*EC2Instances

		synctest.Test(t, func(t *testing.T) {
			watcher := NewWatcher(t.Context(), logtest.NewLogger(), WithPerInstanceHookFn(func(results []*EC2Instances) {
				gotInstances = append(gotInstances, results...)
			}))
			watcher.SetFetchers(noDiscoveryConfig, fetchers)
			go watcher.Run()
			synctest.Wait()
		})

		expectedInstances := []*EC2Instances{
			{
				Region:     "us-west-2",
				Instances:  []EC2Instance{toEC2Instance(present)},
				Parameters: map[string]string{"token": "", "scriptName": ""},
				AccountID:  "123456789012",
			},
			{
				Region:     "us-west-2",
				Instances:  []EC2Instance{toEC2Instance(presentOther)},
				Parameters: map[string]string{"token": "", "scriptName": ""},
				AccountID:  "210987654321",
			},
		}

		require.ElementsMatch(t, expectedInstances, gotInstances)
	})
}

func TestEC2WatcherOrganizationAccessDeniedReturnsPermissionError(t *testing.T) {
	t.Parallel()

	const (
		discoveryConfigName = "dc-org"
		integrationName     = "aws-integration"
		organizationID      = "o-abcdefghij"
		roleARN             = "arn:aws:iam::123456789012:role/OrganizationReader"
	)

	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:       []string{"ec2"},
			Regions:     []string{"us-west-2"},
			Tags:        map[string]utils.Strings{"teleport": {"yes"}},
			Integration: integrationName,
			SSM:         &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleARN:  roleARN,
				RoleName: "OrganizationReader",
			},
			Organization: &types.AWSOrganizationMatcher{
				OrganizationID: organizationID,
				OrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
					Include: []string{types.Wildcard},
				},
			},
		},
	}

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter: func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
			return nil, errors.New("EC2 client getter must not be called when org listing fails")
		},
		AWSOrganizationsGetter: func(ctx context.Context, opts ...awsconfig.OptionsFn) (liborganizations.OrganizationsClient, error) {
			return &mockOrganizationsClient{
				responseError: trace.AccessDenied("organizations denied"),
			}, nil
		},
		DiscoveryConfigName: discoveryConfigName,
	})
	require.NoError(t, err)
	require.Len(t, fetchers, 1)

	results, err := fetchers[0].GetInstances(t.Context(), false)
	require.Error(t, err)
	require.Empty(t, results)

	permissionErrors := EC2IAMPermissionErrors(err)
	require.Len(t, permissionErrors, 1)

	permErr := permissionErrors[0]
	require.Equal(t, integrationName, permErr.Integration)
	require.Equal(t, usertasks.AutoDiscoverEC2IssuePermOrgDenied, permErr.IssueType)
	require.Equal(t, discoveryConfigName, permErr.DiscoveryConfigName)
	require.Equal(t, "123456789012", permErr.AccountID)
	require.Empty(t, permErr.Region)
	require.True(t, trace.IsAccessDenied(permErr.Err))
}

func TestEC2WatcherOrganizationClientCreationPermissionErrorReturnsPermissionError(t *testing.T) {
	t.Parallel()

	const (
		discoveryConfigName = "dc-org-client"
		integrationName     = "aws-integration"
		organizationID      = "o-abcdefghij"
		roleARN             = "arn:aws:iam::123456789012:role/OrganizationReader"
	)

	for _, tt := range []struct {
		name     string
		err      error
		checkErr func(*testing.T, error)
	}{
		{
			name: "access denied",
			err:  trace.AccessDenied("organizations client denied"),
			checkErr: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "invalid identity token",
			err: fmt.Errorf("failed to retrieve credentials: %w", &ststypes.InvalidIdentityTokenException{
				Message: aws.String("No OpenIDConnect provider found"),
			}),
			checkErr: func(t *testing.T, err error) {
				require.ErrorAs(t, err, new(*ststypes.InvalidIdentityTokenException))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matchers := []types.AWSMatcher{
				{
					Params: &types.InstallerParams{
						InstallTeleport: true,
					},
					Types:       []string{"ec2"},
					Regions:     []string{"us-west-2"},
					Tags:        map[string]utils.Strings{"teleport": {"yes"}},
					Integration: integrationName,
					SSM:         &types.AWSSSM{},
					AssumeRole: &types.AssumeRole{
						RoleARN:  roleARN,
						RoleName: "OrganizationReader",
					},
					Organization: &types.AWSOrganizationMatcher{
						OrganizationID: organizationID,
						OrganizationalUnits: &types.AWSOrganizationUnitsMatcher{
							Include: []string{types.Wildcard},
						},
					},
				},
			}

			fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
				Matchers: matchers,
				PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
					return "proxy.example.com:3080", nil
				},
				EC2ClientGetter: func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
					return nil, errors.New("EC2 client getter must not be called when org client creation fails")
				},
				AWSOrganizationsGetter: func(ctx context.Context, opts ...awsconfig.OptionsFn) (liborganizations.OrganizationsClient, error) {
					return nil, tt.err
				},
				DiscoveryConfigName: discoveryConfigName,
			})
			require.NoError(t, err)
			require.Len(t, fetchers, 1)

			results, err := fetchers[0].GetInstances(t.Context(), false)
			require.Error(t, err)
			require.Empty(t, results)

			permissionErrors := EC2IAMPermissionErrors(err)
			require.Len(t, permissionErrors, 1)

			permErr := permissionErrors[0]
			require.Equal(t, integrationName, permErr.Integration)
			require.Equal(t, usertasks.AutoDiscoverEC2IssuePermOrgDenied, permErr.IssueType)
			require.Equal(t, discoveryConfigName, permErr.DiscoveryConfigName)
			require.Equal(t, "123456789012", permErr.AccountID)
			require.Empty(t, permErr.Region)
			tt.checkErr(t, permErr.Err)
		})
	}
}

func TestEC2WatcherListRegionsAccessDeniedReturnsPermissionError(t *testing.T) {
	t.Parallel()

	const (
		discoveryConfigName = "dc-regions"
		integrationName     = "aws-integration"
		roleARN             = "arn:aws:iam::123456789012:role/Discovery"
	)

	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:       []string{"ec2"},
			Regions:     []string{types.Wildcard},
			Tags:        map[string]utils.Strings{"teleport": {"yes"}},
			Integration: integrationName,
			SSM:         &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleARN: roleARN,
			},
		},
	}

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter: func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
			return nil, errors.New("EC2 client getter must not be called when region listing fails")
		},
		RegionsListerGetter: func(ctx context.Context, opts ...awsconfig.OptionsFn) (account.ListRegionsAPIClient, error) {
			return &mockAWSAccountClient{
				responseError: trace.AccessDenied("account regions denied"),
			}, nil
		},
		DiscoveryConfigName: discoveryConfigName,
	})
	require.NoError(t, err)
	require.Len(t, fetchers, 1)

	results, err := fetchers[0].GetInstances(t.Context(), false)
	require.Error(t, err)
	require.Empty(t, results)

	permissionErrors := EC2IAMPermissionErrors(err)
	require.Len(t, permissionErrors, 1)

	permErr := permissionErrors[0]
	require.Equal(t, integrationName, permErr.Integration)
	require.Equal(t, usertasks.AutoDiscoverEC2IssuePermAccountDenied, permErr.IssueType)
	require.Equal(t, discoveryConfigName, permErr.DiscoveryConfigName)
	require.Equal(t, "123456789012", permErr.AccountID)
	require.Empty(t, permErr.Region)
	require.True(t, trace.IsAccessDenied(permErr.Err))
}

func TestEC2WatcherDescribeInstancesAccessDeniedReturnsPermissionErrorAndPartialResults(t *testing.T) {
	t.Parallel()

	const (
		discoveryConfigName = "dc-describe"
		integrationName     = "aws-integration"
		roleARN             = "arn:aws:iam::123456789012:role/Discovery"
	)

	matchers := []types.AWSMatcher{
		{
			Params: &types.InstallerParams{
				InstallTeleport: true,
			},
			Types:       []string{"ec2"},
			Regions:     []string{"us-west-2", "us-east-1"},
			Tags:        map[string]utils.Strings{"teleport": {"yes"}},
			Integration: integrationName,
			SSM:         &types.AWSSSM{},
			AssumeRole: &types.AssumeRole{
				RoleARN: roleARN,
			},
		},
	}

	discoveredInstance := ec2types.Instance{
		InstanceId: aws.String("instance-present"),
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

	fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
		Matchers: matchers,
		PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
			return "proxy.example.com:3080", nil
		},
		EC2ClientGetter: func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
			if region == "us-west-2" {
				return &mockEC2Client{
					responseError: trace.AccessDenied("describe instances denied"),
				}, nil
			}
			return &mockEC2Client{
				output: &ec2.DescribeInstancesOutput{
					Reservations: []ec2types.Reservation{
						{
							OwnerId:   aws.String("123456789012"),
							Instances: []ec2types.Instance{discoveredInstance},
						},
					},
				},
			}, nil
		},
		DiscoveryConfigName: discoveryConfigName,
	})
	require.NoError(t, err)
	require.Len(t, fetchers, 1)

	results, err := fetchers[0].GetInstances(t.Context(), false)
	require.Error(t, err)

	permissionErrors := EC2IAMPermissionErrors(err)
	require.Len(t, permissionErrors, 1)

	permErr := permissionErrors[0]
	require.Equal(t, integrationName, permErr.Integration)
	require.Equal(t, usertasks.AutoDiscoverEC2IssuePermAccountDenied, permErr.IssueType)
	require.Equal(t, discoveryConfigName, permErr.DiscoveryConfigName)
	require.Equal(t, "123456789012", permErr.AccountID)
	require.Equal(t, "us-west-2", permErr.Region)
	require.True(t, trace.IsAccessDenied(permErr.Err))

	require.Len(t, results, 1)
	require.Equal(t, &EC2Instances{
		AccountID:           "123456789012",
		Region:              "us-east-1",
		DocumentName:        "",
		Instances:           []EC2Instance{toEC2Instance(discoveredInstance)},
		Parameters:          map[string]string{"token": "", "scriptName": ""},
		Integration:         integrationName,
		AssumeRoleARN:       roleARN,
		DiscoveryConfigName: discoveryConfigName,
	}, results[0])
}

func TestEC2WatcherClientCreationPermissionErrorReturnsPermissionError(t *testing.T) {
	t.Parallel()

	const (
		discoveryConfigName = "dc-client"
		integrationName     = "aws-integration"
		roleARN             = "arn:aws:iam::123456789012:role/Discovery"
		region              = "us-west-2"
	)

	for _, tt := range []struct {
		name     string
		err      error
		checkErr func(*testing.T, error)
	}{
		{
			name: "access denied",
			err:  trace.AccessDenied("ec2 client denied"),
			checkErr: func(t *testing.T, err error) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "invalid identity token",
			err: fmt.Errorf("failed to retrieve credentials: %w", &ststypes.InvalidIdentityTokenException{
				Message: aws.String("No OpenIDConnect provider found"),
			}),
			checkErr: func(t *testing.T, err error) {
				require.ErrorAs(t, err, new(*ststypes.InvalidIdentityTokenException))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matchers := []types.AWSMatcher{
				{
					Params: &types.InstallerParams{
						InstallTeleport: true,
					},
					Types:       []string{"ec2"},
					Regions:     []string{region},
					Tags:        map[string]utils.Strings{"teleport": {"yes"}},
					Integration: integrationName,
					SSM:         &types.AWSSSM{},
					AssumeRole: &types.AssumeRole{
						RoleARN: roleARN,
					},
				},
			}

			fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), MatcherToEC2FetcherParams{
				Matchers: matchers,
				PublicProxyAddrGetter: func(ctx context.Context) (string, error) {
					return "proxy.example.com:3080", nil
				},
				EC2ClientGetter: func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
					return nil, tt.err
				},
				DiscoveryConfigName: discoveryConfigName,
			})
			require.NoError(t, err)
			require.Len(t, fetchers, 1)

			results, err := fetchers[0].GetInstances(t.Context(), false)
			require.Error(t, err)
			require.Empty(t, results)

			permissionErrors := EC2IAMPermissionErrors(err)
			require.Len(t, permissionErrors, 1)

			permErr := permissionErrors[0]
			require.Equal(t, integrationName, permErr.Integration)
			require.Equal(t, usertasks.AutoDiscoverEC2IssuePermAccountDenied, permErr.IssueType)
			require.Equal(t, discoveryConfigName, permErr.DiscoveryConfigName)
			require.Equal(t, "123456789012", permErr.AccountID)
			require.Equal(t, region, permErr.Region)
			tt.checkErr(t, permErr.Err)
		})
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

	const noDiscoveryConfig = ""
	watcher := NewWatcher[*EC2Instances](t.Context(), logtest.NewLogger())
	watcher.SetFetchers(noDiscoveryConfig, fetchers)

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

	var gotInstances []*EC2Instances
	for len(gotInstances) < len(expectedInstances) {
		select {
		case instances := <-watcher.InstancesC:
			require.NotNil(t, instances)
			gotInstances = append(gotInstances, instances)
		case <-t.Context().Done():
			require.Fail(t, "context canceled")
		}
	}
	require.ElementsMatch(t, expectedInstances, gotInstances)

	select {
	case result := <-watcher.InstancesC:
		require.Fail(t, "unexpected result: %v", result)
	default:
	}
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
