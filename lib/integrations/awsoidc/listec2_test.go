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

package awsoidc

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockListEC2Client struct {
	pageSize     int
	accountID    string
	ec2Instances []ec2Types.Instance
}

// GetCallerIdentity returns information about the caller identity.
func (m *mockListEC2Client) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: &m.accountID,
	}, nil
}

// Returns information about ec2 instances.
// This API supports pagination.
func (m mockListEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	requestedPage := 1

	stateFilter := false
	for _, filter := range params.Filters {
		if aws.ToString(filter.Name) == "instance-state-name" && len(filter.Values) == 1 && filter.Values[0] == "running" {
			stateFilter = true
		}
	}
	if !stateFilter {
		return nil, trace.BadParameter("instance-state-name filter was not included")
	}

	totalInstances := len(m.ec2Instances)

	if params.NextToken != nil {
		currentMarker, err := strconv.Atoi(*params.NextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := m.pageSize * (requestedPage - 1)
	sliceEnd := m.pageSize * requestedPage
	if sliceEnd > totalInstances {
		sliceEnd = totalInstances
	}

	ret := &ec2.DescribeInstancesOutput{
		Reservations: []ec2Types.Reservation{{
			Instances: m.ec2Instances[sliceStart:sliceEnd],
		}},
	}

	if sliceEnd < totalInstances {
		nextToken := strconv.Itoa(requestedPage + 1)
		ret.NextToken = stringPointer(nextToken)
	}

	return ret, nil
}

func TestListEC2(t *testing.T) {
	ctx := context.Background()

	noErrorFunc := func(err error) bool {
		return err == nil
	}

	const pageSize = 100
	t.Run("pagination", func(t *testing.T) {
		totalEC2s := 203

		allInstances := make([]ec2Types.Instance, 0, totalEC2s)
		for i := 0; i < totalEC2s; i++ {
			allInstances = append(allInstances, ec2Types.Instance{
				PrivateDnsName:   aws.String("my-private-dns.compute.aws"),
				InstanceId:       aws.String(fmt.Sprintf("i-12345678%d", i)),
				VpcId:            aws.String("vpc-abcd"),
				SubnetId:         aws.String("subnet-123"),
				PrivateIpAddress: aws.String("172.31.1.1"),
			})
		}

		mockListClient := &mockListEC2Client{
			pageSize:     pageSize,
			accountID:    "123456789012",
			ec2Instances: allInstances,
		}

		// First page must return pageSize number of Servers
		resp, err := ListEC2(ctx, mockListClient, ListEC2Request{
			Region:      "us-east-1",
			Integration: "myintegration",
			NextToken:   "",
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.Servers, pageSize)
		nextPageToken := resp.NextToken
		require.Equal(t, "i-123456780", resp.Servers[0].GetCloudMetadata().AWS.InstanceID)

		// Second page must return pageSize number of Servers
		resp, err = ListEC2(ctx, mockListClient, ListEC2Request{
			Region:      "us-east-1",
			Integration: "myintegration",
			NextToken:   nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.Servers, pageSize)
		nextPageToken = resp.NextToken
		require.Equal(t, "i-12345678100", resp.Servers[0].GetCloudMetadata().AWS.InstanceID)

		// Third page must return only the remaining Servers and an empty nextToken
		resp, err = ListEC2(ctx, mockListClient, ListEC2Request{
			Region:      "us-east-1",
			Integration: "myintegration",
			NextToken:   nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Len(t, resp.Servers, 3)
		require.Equal(t, "i-12345678200", resp.Servers[0].GetCloudMetadata().AWS.InstanceID)
	})

	for _, tt := range []struct {
		name            string
		req             ListEC2Request
		mockInstances   []ec2Types.Instance
		defaultPageSize int
		errCheck        func(error) bool
		respCheck       func(*testing.T, *ListEC2Response)
	}{
		{
			name: "valid for listing instances",
			req: ListEC2Request{
				Region:      "us-east-1",
				Integration: "myintegration",
				NextToken:   "",
			},
			mockInstances: []ec2Types.Instance{{
				PrivateDnsName:   aws.String("my-private-dns.compute.aws"),
				InstanceId:       aws.String("i-123456789abcedf"),
				VpcId:            aws.String("vpc-abcd"),
				SubnetId:         aws.String("subnet-123"),
				PrivateIpAddress: aws.String("172.31.1.1"),
			},
			},
			respCheck: func(t *testing.T, ldr *ListEC2Response) {
				require.Len(t, ldr.Servers, 1, "expected 1 server, got %d", len(ldr.Servers))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				expectedServer := &types.ServerV2{
					Kind:    "node",
					Version: "v2",
					SubKind: "openssh-ec2-ice",
					Metadata: types.Metadata{
						Labels: map[string]string{
							"account-id":               "123456789012",
							"region":                   "us-east-1",
							"teleport.dev/instance-id": "i-123456789abcedf",
							"teleport.dev/account-id":  "123456789012",
						},
						Namespace: "default",
					},
					Spec: types.ServerSpecV2{
						Addr:     "172.31.1.1:22",
						Hostname: "my-private-dns.compute.aws",
						CloudMetadata: &types.CloudMetadata{
							AWS: &types.AWSInfo{
								AccountID:   "123456789012",
								InstanceID:  "i-123456789abcedf",
								Region:      "us-east-1",
								VPCID:       "vpc-abcd",
								Integration: "myintegration",
								SubnetID:    "subnet-123",
							},
						},
					},
				}
				require.Empty(t, cmp.Diff(expectedServer, ldr.Servers[0], cmpopts.IgnoreFields(types.ServerV2{}, "Metadata.Name")))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "valid but all instances are windows",
			req: ListEC2Request{
				Region:      "us-east-1",
				Integration: "myintegration",
				NextToken:   "",
			},
			mockInstances: []ec2Types.Instance{{
				PrivateDnsName:   aws.String("my-private-dns.compute.aws"),
				InstanceId:       aws.String("i-123456789abcedf"),
				VpcId:            aws.String("vpc-abcd"),
				SubnetId:         aws.String("subnet-123"),
				PrivateIpAddress: aws.String("172.31.1.1"),
				Platform:         "windows",
			}},
			respCheck: func(t *testing.T, ldr *ListEC2Response) {
				require.Empty(t, ldr.Servers)
			},
			errCheck: noErrorFunc,
		},
		{
			name: "valid but some instances are windows, it ensures the page is never empty",
			req: ListEC2Request{
				Region:      "us-east-1",
				Integration: "myintegration",
				NextToken:   "",
			},
			defaultPageSize: 2,
			mockInstances: []ec2Types.Instance{
				{
					PrivateDnsName:   aws.String("my-private-dns.compute.aws"),
					InstanceId:       aws.String("i-123456789abcedf"),
					VpcId:            aws.String("vpc-abcd"),
					SubnetId:         aws.String("subnet-123"),
					PrivateIpAddress: aws.String("172.31.1.1"),
					Platform:         "windows",
				},
				{
					PrivateDnsName:   aws.String("my-private-dns.compute.aws"),
					InstanceId:       aws.String("i-123456789abcedf"),
					VpcId:            aws.String("vpc-abcd"),
					SubnetId:         aws.String("subnet-123"),
					PrivateIpAddress: aws.String("172.31.1.1"),
					Platform:         "windows",
				},
				{
					PrivateDnsName:   aws.String("my-private-dns.compute.aws"),
					InstanceId:       aws.String("i-123456789abcedf"),
					VpcId:            aws.String("vpc-abcd"),
					SubnetId:         aws.String("subnet-123"),
					PrivateIpAddress: aws.String("172.31.1.1"),
				}},
			respCheck: func(t *testing.T, ldr *ListEC2Response) {
				require.Len(t, ldr.Servers, 1)
			},
			errCheck: noErrorFunc,
		},
		{
			name: "no region",
			req: ListEC2Request{
				Integration: "myintegration",
			},
			errCheck: trace.IsBadParameter,
		},
		{
			name: "no integration",
			req: ListEC2Request{
				Region: "us-east-1",
			},
			errCheck: trace.IsBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockPageSize := tt.defaultPageSize
			if tt.defaultPageSize == 0 {
				mockPageSize = pageSize
			}
			mockListClient := &mockListEC2Client{
				pageSize:     mockPageSize,
				accountID:    "123456789012",
				ec2Instances: tt.mockInstances,
			}
			resp, err := ListEC2(ctx, mockListClient, tt.req)
			require.True(t, tt.errCheck(err), "unexpected err: %v", err)
			if tt.respCheck != nil {
				tt.respCheck(t, resp)
			}
		})
	}
}
