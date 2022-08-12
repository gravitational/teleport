/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
)

type mockClients struct {
	cloud.Clients

	ec2Client *mockEC2Client
}

func (c *mockClients) GetAWSEC2Client(region string) (ec2iface.EC2API, error) {
	return c.ec2Client, nil
}

type mockEC2Client struct {
	ec2iface.EC2API
	output *ec2.DescribeInstancesOutput
}

func (m *mockEC2Client) DescribeInstancesPagesWithContext(
	ctx context.Context, input *ec2.DescribeInstancesInput,
	f func(dio *ec2.DescribeInstancesOutput, b bool) bool, opts ...request.Option) error {
	f(m.output, true)
	return nil
}

func TestEC2Watcher(t *testing.T) {
	clients := mockClients{
		ec2Client: &mockEC2Client{},
	}
	matchers := []services.AWSMatcher{
		{
			Types:   []string{"EC2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"teleport": {"yes"}},
		},
		{
			Types:   []string{"EC2"},
			Regions: []string{"us-west-2"},
			Tags:    map[string]utils.Strings{"env": {"dev"}},
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

	output := ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{
			Instances: []*ec2.Instance{
				&present,
				&presentOther,
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
	watcher, err := NewCloudWatcher(ctx, matchers, &clients)
	require.NoError(t, err)

	go watcher.Run()

	result := <-watcher.InstancesC
	require.Equal(t, EC2Instances{
		Region:    "us-west-2",
		Instances: []*ec2.Instance{&present},
	}, result)
	result = <-watcher.InstancesC
	require.Equal(t, EC2Instances{
		Region:    "us-west-2",
		Instances: []*ec2.Instance{&presentOther},
	}, result)
}
