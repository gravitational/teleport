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
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockListEC2ICEClient struct {
	pageSize  int
	accountID string
	ec2ICEs   []ec2Types.Ec2InstanceConnectEndpoint
}

// Returns information about ec2 instances.
// This API supports pagination.
func (m mockListEC2ICEClient) DescribeInstanceConnectEndpoints(ctx context.Context, params *ec2.DescribeInstanceConnectEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceConnectEndpointsOutput, error) {
	requestedPage := 1

	totalEndpoints := len(m.ec2ICEs)

	if params.NextToken != nil {
		currentMarker, err := strconv.Atoi(*params.NextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := m.pageSize * (requestedPage - 1)
	sliceEnd := m.pageSize * requestedPage
	if sliceEnd > totalEndpoints {
		sliceEnd = totalEndpoints
	}

	ret := &ec2.DescribeInstanceConnectEndpointsOutput{
		InstanceConnectEndpoints: m.ec2ICEs[sliceStart:sliceEnd],
	}

	if sliceEnd < totalEndpoints {
		nextToken := strconv.Itoa(requestedPage + 1)
		ret.NextToken = &nextToken
	}

	return ret, nil
}

func TestListEC2ICE(t *testing.T) {
	ctx := context.Background()

	noErrorFunc := func(err error) bool {
		return err == nil
	}

	const pageSize = 100
	t.Run("pagination", func(t *testing.T) {
		totalEC2ICEs := 203

		allEndpoints := make([]ec2Types.Ec2InstanceConnectEndpoint, 0, totalEC2ICEs)
		for i := 0; i < totalEC2ICEs; i++ {
			allEndpoints = append(allEndpoints, ec2Types.Ec2InstanceConnectEndpoint{
				SubnetId:                  aws.String(fmt.Sprintf("subnet-%d", i)),
				InstanceConnectEndpointId: aws.String("ice-name"),
				State:                     "create-complete",
			})
		}

		mockListClient := &mockListEC2ICEClient{
			pageSize:  pageSize,
			accountID: "123456789012",
			ec2ICEs:   allEndpoints,
		}

		// First page must return pageSize number of Endpoints
		resp, err := ListEC2ICE(ctx, mockListClient, ListEC2ICERequest{
			VPCIDs:    []string{"vpc-123"},
			Region:    "us-east-1",
			NextToken: "",
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#Endpoints:v=3;vpcEndpointType=EC2%20Instance%20Connect%20Endpoint", resp.DashboardLink)
		require.Len(t, resp.EC2ICEs, pageSize)
		nextPageToken := resp.NextToken
		require.Equal(t, "subnet-0", resp.EC2ICEs[0].SubnetID)

		// Second page must return pageSize number of Endpoints
		resp, err = ListEC2ICE(ctx, mockListClient, ListEC2ICERequest{
			VPCIDs:    []string{"vpc-abc"},
			Region:    "us-east-1",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#Endpoints:v=3;vpcEndpointType=EC2%20Instance%20Connect%20Endpoint", resp.DashboardLink)
		require.Len(t, resp.EC2ICEs, pageSize)
		nextPageToken = resp.NextToken
		require.Equal(t, "subnet-100", resp.EC2ICEs[0].SubnetID)

		// Third page must return only the remaining Endpoints and an empty nextToken
		resp, err = ListEC2ICE(ctx, mockListClient, ListEC2ICERequest{
			VPCIDs:    []string{"vpc-abc"},
			Region:    "us-east-1",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#Endpoints:v=3;vpcEndpointType=EC2%20Instance%20Connect%20Endpoint", resp.DashboardLink)
		require.Len(t, resp.EC2ICEs, 3)
		require.Equal(t, "subnet-200", resp.EC2ICEs[0].SubnetID)
	})

	for _, tt := range []struct {
		name          string
		req           ListEC2ICERequest
		mockEndpoints []ec2Types.Ec2InstanceConnectEndpoint
		errCheck      func(error) bool
		respCheck     func(*testing.T, *ListEC2ICEResponse)
	}{
		{
			name: "valid for listing endpoints",
			req: ListEC2ICERequest{
				VPCIDs:    []string{"vpc-abcd"},
				Region:    "us-east-1",
				NextToken: "",
			},
			mockEndpoints: []ec2Types.Ec2InstanceConnectEndpoint{{
				SubnetId:                  aws.String("subnet-123"),
				VpcId:                     aws.String("vpc-abcd"),
				InstanceConnectEndpointId: aws.String("ice-name"),
				State:                     "create-complete",
				StateMessage:              aws.String("success message"),
			},
			},
			respCheck: func(t *testing.T, ldr *ListEC2ICEResponse) {
				require.Len(t, ldr.EC2ICEs, 1, "expected 1 endpoint, got %d", len(ldr.EC2ICEs))
				require.Equal(t, "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#Endpoints:v=3;vpcEndpointType=EC2%20Instance%20Connect%20Endpoint", ldr.DashboardLink)
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				endpoint := EC2InstanceConnectEndpoint{
					Name:          "ice-name",
					State:         "create-complete",
					SubnetID:      "subnet-123",
					VPCID:         "vpc-abcd",
					StateMessage:  "success message",
					DashboardLink: "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#InstanceConnectEndpointDetails:instanceConnectEndpointId=ice-name",
				}
				require.Empty(t, cmp.Diff(endpoint, ldr.EC2ICEs[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "valid but ID needs URL encoding",
			req: ListEC2ICERequest{
				VPCIDs:    []string{"vpc-abcd"},
				Region:    "us-east-1",
				NextToken: "",
			},
			mockEndpoints: []ec2Types.Ec2InstanceConnectEndpoint{{
				SubnetId:                  aws.String("subnet-123"),
				InstanceConnectEndpointId: aws.String("ice/123"),
				State:                     "create-complete",
				StateMessage:              aws.String("success message"),
			},
			},
			respCheck: func(t *testing.T, ldr *ListEC2ICEResponse) {
				require.Len(t, ldr.EC2ICEs, 1, "expected 1 endpoint, got %d", len(ldr.EC2ICEs))
				require.Equal(t, "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#Endpoints:v=3;vpcEndpointType=EC2%20Instance%20Connect%20Endpoint", ldr.DashboardLink)
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				endpoint := EC2InstanceConnectEndpoint{
					Name:          "ice/123",
					State:         "create-complete",
					SubnetID:      "subnet-123",
					StateMessage:  "success message",
					DashboardLink: "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#InstanceConnectEndpointDetails:instanceConnectEndpointId=ice%2F123",
				}
				require.Empty(t, cmp.Diff(endpoint, ldr.EC2ICEs[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "valid for multiple VPCs",
			req: ListEC2ICERequest{
				VPCIDs:    []string{"vpc-01", "vpc-02"},
				Region:    "us-east-1",
				NextToken: "",
			},
			mockEndpoints: []ec2Types.Ec2InstanceConnectEndpoint{
				{
					SubnetId:                  aws.String("subnet-123"),
					VpcId:                     aws.String("vpc-01"),
					InstanceConnectEndpointId: aws.String("ice-name-1"),
					State:                     "create-complete",
					StateMessage:              aws.String("success message"),
				},
				{
					SubnetId:                  aws.String("subnet-123"),
					VpcId:                     aws.String("vpc-02"),
					InstanceConnectEndpointId: aws.String("ice-name-2"),
					State:                     "create-complete",
					StateMessage:              aws.String("success message"),
				},
			},
			respCheck: func(t *testing.T, ldr *ListEC2ICEResponse) {
				require.Len(t, ldr.EC2ICEs, 2, "expected 1 endpoint, got %d", len(ldr.EC2ICEs))
				require.Equal(t, "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#Endpoints:v=3;vpcEndpointType=EC2%20Instance%20Connect%20Endpoint", ldr.DashboardLink)
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				endpoint := EC2InstanceConnectEndpoint{
					Name:          "ice-name-1",
					State:         "create-complete",
					SubnetID:      "subnet-123",
					VPCID:         "vpc-01",
					StateMessage:  "success message",
					DashboardLink: "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#InstanceConnectEndpointDetails:instanceConnectEndpointId=ice-name-1",
				}
				require.Empty(t, cmp.Diff(endpoint, ldr.EC2ICEs[0]))

				endpoint = EC2InstanceConnectEndpoint{
					Name:          "ice-name-2",
					State:         "create-complete",
					SubnetID:      "subnet-123",
					VPCID:         "vpc-02",
					StateMessage:  "success message",
					DashboardLink: "https://us-east-1.console.aws.amazon.com/vpcconsole/home?#InstanceConnectEndpointDetails:instanceConnectEndpointId=ice-name-2",
				}
				require.Empty(t, cmp.Diff(endpoint, ldr.EC2ICEs[1]))
			},
			errCheck: noErrorFunc,
		},
		{
			name: "no vpc id",
			req: ListEC2ICERequest{
				Region: "us-east-1",
			},
			errCheck: trace.IsBadParameter,
		},
		{
			name: "no region id",
			req: ListEC2ICERequest{
				VPCIDs: []string{"vpc-123"},
			},
			errCheck: trace.IsBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockListClient := &mockListEC2ICEClient{
				pageSize:  pageSize,
				accountID: "123456789012",
				ec2ICEs:   tt.mockEndpoints,
			}
			resp, err := ListEC2ICE(ctx, mockListClient, tt.req)
			require.True(t, tt.errCheck(err), "unexpected err: %v", err)
			if tt.respCheck != nil {
				tt.respCheck(t, resp)
			}
		})
	}
}
