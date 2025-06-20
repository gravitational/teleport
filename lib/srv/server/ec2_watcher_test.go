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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

type mockEC2Client struct {
	output *ec2.DescribeInstancesOutput
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
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

func makeMockClients(m map[string]*ec2.DescribeInstancesOutput) EC2ClientGetter {
	return func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (ec2.DescribeInstancesAPIClient, error) {
		var options awsconfig.Options
		for _, opt := range opts {
			opt(&options)
		}
		var roleARN string
		if len(options.AssumeRoles) != 0 {
			roleARN = options.AssumeRoles[len(options.AssumeRoles)-1].RoleARN
		}
		return &mockEC2Client{
			output: m[roleARN],
		}, nil
	}
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
				Labels: types.Labels{
					"*":     []string{},
					"hello": []string{"other"},
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
				Labels: types.Labels{
					"hello": []string{"other"},
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

	output := ec2.DescribeInstancesOutput{
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
	altAccountOutput := ec2.DescribeInstancesOutput{
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
	getClient := makeMockClients(map[string]*ec2.DescribeInstancesOutput{
		"":                   &output,
		"alternate-role-arn": &altAccountOutput,
	})

	const noDiscoveryConfig = ""
	fetchersFn := func() []Fetcher {
		fetchers, err := MatchersToEC2InstanceFetchers(t.Context(), matchers, getClient, noDiscoveryConfig)
		require.NoError(t, err)

		return fetchers
	}
	watcher, err := NewEC2Watcher(t.Context(), fetchersFn, make(<-chan []types.Server))
	require.NoError(t, err)

	go watcher.Run()

	expectedInstances := []EC2Instances{
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
	}

	for _, instances := range expectedInstances {
		select {
		case result := <-watcher.InstancesC:
			require.NotNil(t, result.EC2)
			require.Equal(t, instances, *result.EC2)
		case <-t.Context().Done():
			require.Fail(t, "context canceled")
		}
	}

	select {
	case inst := <-watcher.InstancesC:
		require.Fail(t, "unexpected instance: %v", inst)
	default:
	}
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
