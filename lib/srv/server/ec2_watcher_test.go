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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

type mockClients struct {
	cloud.Clients

	ec2Client   *mockEC2Client
	azureClient azure.VirtualMachinesClient
}

func (c *mockClients) GetAWSEC2Client(ctx context.Context, region string, _ ...cloud.AWSAssumeRoleOptionFn) (ec2iface.EC2API, error) {
	return c.ec2Client, nil
}

type mockEC2Client struct {
	ec2iface.EC2API
	output *ec2.DescribeInstancesOutput
}

func instanceMatches(inst *ec2.Instance, filters []*ec2.Filter) bool {
	allMatched := true
	for _, filter := range filters {
		name := aws.StringValue(filter.Name)
		val := aws.StringValue(filter.Values[0])
		if name == AWSInstanceStateName && aws.StringValue(inst.State.Name) != ec2.InstanceStateNameRunning {
			return false
		}
		for _, tag := range inst.Tags {
			if aws.StringValue(tag.Key) != name[4:] {
				continue
			}
			allMatched = allMatched && aws.StringValue(tag.Value) != val
		}
	}

	return !allMatched
}

func (m *mockEC2Client) DescribeInstancesPagesWithContext(
	ctx context.Context, input *ec2.DescribeInstancesInput,
	f func(dio *ec2.DescribeInstancesOutput, b bool) bool, opts ...request.Option) error {
	output := &ec2.DescribeInstancesOutput{}
	for _, res := range m.output.Reservations {
		var instances []*ec2.Instance
		for _, inst := range res.Instances {
			if instanceMatches(inst, input.Filters) {
				instances = append(instances, inst)
			}
		}
		output.Reservations = append(output.Reservations, &ec2.Reservation{
			Instances: instances,
		})
	}

	f(output, true)
	return nil
}

func TestNewEC2InstanceFetcherTags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		config          ec2FetcherConfig
		expectedFilters []*ec2.Filter
	}{
		{
			name: "with glob key",
			config: ec2FetcherConfig{
				Labels: types.Labels{
					"*":     []string{},
					"hello": []string{"other"},
				},
			},
			expectedFilters: []*ec2.Filter{
				{
					Name:   aws.String(AWSInstanceStateName),
					Values: aws.StringSlice([]string{ec2.InstanceStateNameRunning}),
				},
			},
		},
		{
			name: "with no glob key",
			config: ec2FetcherConfig{
				Labels: types.Labels{
					"hello": []string{"other"},
				},
			},
			expectedFilters: []*ec2.Filter{
				{
					Name:   aws.String(AWSInstanceStateName),
					Values: aws.StringSlice([]string{ec2.InstanceStateNameRunning}),
				},
				{
					Name:   aws.String("tag:hello"),
					Values: aws.StringSlice([]string{"other"}),
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
	clients := mockClients{
		ec2Client: &mockEC2Client{},
	}
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
			Types:       []string{"EC2"},
			Regions:     []string{"us-west-2"},
			Tags:        map[string]utils.Strings{"with-eice": {"please"}},
			Integration: "my-aws-integration",
			SSM:         &types.AWSSSM{},
		},
	}
	ctx := context.Background()

	present := ec2.Instance{
		InstanceId: aws.String("instance-present"),
		Tags: []*ec2.Tag{{
			Key:   aws.String("teleport"),
			Value: aws.String("yes"),
		}},
		State: &ec2.InstanceState{
			Name: aws.String(ec2.InstanceStateNameRunning),
		},
	}
	presentOther := ec2.Instance{
		InstanceId: aws.String("instance-present-2"),
		Tags: []*ec2.Tag{{
			Key:   aws.String("env"),
			Value: aws.String("dev"),
		}},
		State: &ec2.InstanceState{
			Name: aws.String(ec2.InstanceStateNameRunning),
		},
	}
	presentForEICE := ec2.Instance{
		InstanceId: aws.String("instance-present-3"),
		Tags: []*ec2.Tag{{
			Key:   aws.String("with-eice"),
			Value: aws.String("please"),
		}},
		State: &ec2.InstanceState{
			Name: aws.String(ec2.InstanceStateNameRunning),
		},
	}

	output := ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{
			Instances: []*ec2.Instance{
				&present,
				&presentOther,
				&presentForEICE,
				{
					InstanceId: aws.String("instance-absent"),
					Tags: []*ec2.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("prod"),
					}},
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
				},
				{
					InstanceId: aws.String("instance-absent-3"),
					Tags: []*ec2.Tag{{
						Key:   aws.String("env"),
						Value: aws.String("prod"),
					}, {
						Key:   aws.String("teleport"),
						Value: aws.String("yes"),
					}},
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNamePending),
					},
				},
			},
		}},
	}
	clients.ec2Client.output = &output

	fetchersFn := func() []Fetcher {
		fetchers, err := MatchersToEC2InstanceFetchers(ctx, matchers, &clients)
		require.NoError(t, err)

		return fetchers
	}
	watcher, err := NewEC2Watcher(ctx, fetchersFn, make(<-chan []types.Server))
	require.NoError(t, err)

	go watcher.Run()

	result := <-watcher.InstancesC
	require.Equal(t, EC2Instances{
		Region:     "us-west-2",
		Instances:  []EC2Instance{toEC2Instance(&present)},
		Parameters: map[string]string{"token": "", "scriptName": ""},
	}, *result.EC2)
	result = <-watcher.InstancesC
	require.Equal(t, EC2Instances{
		Region:     "us-west-2",
		Instances:  []EC2Instance{toEC2Instance(&presentOther)},
		Parameters: map[string]string{"token": "", "scriptName": ""},
	}, *result.EC2)
	result = <-watcher.InstancesC
	require.Equal(t, EC2Instances{
		Region:      "us-west-2",
		Instances:   []EC2Instance{toEC2Instance(&presentForEICE)},
		Parameters:  map[string]string{"token": "", "scriptName": "", "sshdConfigPath": ""},
		Integration: "my-aws-integration",
	}, *result.EC2)
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
