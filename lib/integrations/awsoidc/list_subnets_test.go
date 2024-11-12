/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

var _ ListSubnetsClient = (*mockListSubnetsClient)(nil)

type mockListSubnetsClient struct {
	pageSize int
	subnets  []ec2Types.Subnet
}

// Returns information about AWS VPC subnets.
// This API supports pagination.
func (m mockListSubnetsClient) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	requestedPage := 1

	totalSubnets := len(m.subnets)

	if params.NextToken != nil {
		currentMarker, err := strconv.Atoi(*params.NextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := m.pageSize * (requestedPage - 1)
	sliceEnd := m.pageSize * requestedPage
	if sliceEnd > totalSubnets {
		sliceEnd = totalSubnets
	}

	ret := &ec2.DescribeSubnetsOutput{
		Subnets: m.subnets[sliceStart:sliceEnd],
	}

	if sliceEnd < totalSubnets {
		nextToken := strconv.Itoa(requestedPage + 1)
		ret.NextToken = &nextToken
	}

	return ret, nil
}

func TestListSubnets(t *testing.T) {
	ctx := context.Background()

	noErrorFunc := func(err error) bool {
		return err == nil
	}

	const pageSize = 100
	t.Run("pagination", func(t *testing.T) {
		totalSubnets := 203

		subnets := make([]ec2Types.Subnet, 0, totalSubnets)
		for i := 0; i < totalSubnets; i++ {
			subnets = append(subnets, ec2Types.Subnet{
				SubnetId: aws.String(fmt.Sprintf("subnet-%d", i)),
				Tags:     makeNameTags(fmt.Sprintf("MySubnet-%d", i)),
			})
		}

		mockListClient := &mockListSubnetsClient{
			pageSize: pageSize,
			subnets:  subnets,
		}

		// First page must return pageSize number of subnets
		resp, err := ListSubnets(ctx, mockListClient, ListSubnetsRequest{
			VPCID:     "vpc-123",
			NextToken: "",
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.Subnets, pageSize)
		nextPageToken := resp.NextToken
		require.Equal(t, "subnet-0", resp.Subnets[0].ID)
		require.Equal(t, "MySubnet-0", resp.Subnets[0].Name)

		// Second page must return pageSize number of Endpoints
		resp, err = ListSubnets(ctx, mockListClient, ListSubnetsRequest{
			VPCID:     "vpc-123",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.Subnets, pageSize)
		nextPageToken = resp.NextToken
		require.Equal(t, "subnet-100", resp.Subnets[0].ID)
		require.Equal(t, "MySubnet-100", resp.Subnets[0].Name)

		// Third page must return only the remaining Endpoints and an empty nextToken
		resp, err = ListSubnets(ctx, mockListClient, ListSubnetsRequest{
			VPCID:     "vpc-123",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Len(t, resp.Subnets, 3)
		require.Equal(t, "subnet-200", resp.Subnets[0].ID)
		require.Equal(t, "MySubnet-200", resp.Subnets[0].Name)
	})

	for _, tt := range []struct {
		name        string
		req         ListSubnetsRequest
		mockSubnets []ec2Types.Subnet
		errCheck    func(error) bool
		respCheck   func(*testing.T, *ListSubnetsResponse)
	}{
		{
			name: "valid for listing subnets",
			req: ListSubnetsRequest{
				VPCID:     "vpc-abc",
				NextToken: "",
			},
			mockSubnets: []ec2Types.Subnet{{
				AvailabilityZone: aws.String("us-west-1a"),
				SubnetId:         aws.String("subnet-123"),
				Tags:             makeNameTags("MySubnet-123"),
			}},
			respCheck: func(t *testing.T, ldr *ListSubnetsResponse) {
				require.Len(t, ldr.Subnets, 1)
				require.Empty(t, ldr.NextToken, "there is only 1 page of subnets")

				want := Subnet{
					ID:               "subnet-123",
					Name:             "MySubnet-123",
					AvailabilityZone: "us-west-1a",
				}
				require.Empty(t, cmp.Diff(want, ldr.Subnets[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name:     "no vpc id",
			req:      ListSubnetsRequest{},
			errCheck: trace.IsBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockListClient := &mockListSubnetsClient{
				pageSize: pageSize,
				subnets:  tt.mockSubnets,
			}
			resp, err := ListSubnets(ctx, mockListClient, tt.req)
			require.True(t, tt.errCheck(err), "unexpected err: %v", err)
			if tt.respCheck != nil {
				tt.respCheck(t, resp)
			}
		})
	}
}

func TestConvertSubnet(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    []ec2Types.Subnet
		expected []Subnet
	}{
		{
			name: "no name tag",
			input: []ec2Types.Subnet{{
				SubnetId: aws.String("subnet-abc"),
				Tags: []ec2Types.Tag{
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
				AvailabilityZone: aws.String("us-east-1a"),
			}},
			expected: []Subnet{{
				Name:             "",
				ID:               "subnet-abc",
				AvailabilityZone: "us-east-1a",
			}},
		},
		{
			name: "with name tag",
			input: []ec2Types.Subnet{{
				SubnetId: aws.String("subnet-abc"),
				Tags: []ec2Types.Tag{
					{Key: aws.String("foo"), Value: aws.String("bar")},
					{Key: aws.String("Name"), Value: aws.String("llama")},
					{Key: aws.String("baz"), Value: aws.String("qux")},
				},
				AvailabilityZone: aws.String("us-east-1a"),
			}},
			expected: []Subnet{{
				Name:             "llama",
				ID:               "subnet-abc",
				AvailabilityZone: "us-east-1a",
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, convertAWSSubnets(tt.input))
		})
	}
}

func makeNameTags(name string) []ec2Types.Tag {
	return []ec2Types.Tag{{
		Key:   aws.String("Name"),
		Value: aws.String(name),
	}}
}
