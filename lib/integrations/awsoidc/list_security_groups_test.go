/*
Copyright 2023 Gravitational, Inc.

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

type mockListSecurityGroupsClient struct {
	pageSize int
	sgs      []ec2Types.SecurityGroup
}

// Returns information about ec2 instances.
// This API supports pagination.
func (m mockListSecurityGroupsClient) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	requestedPage := 1

	totalSG := len(m.sgs)

	if params.NextToken != nil {
		currentMarker, err := strconv.Atoi(*params.NextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := m.pageSize * (requestedPage - 1)
	sliceEnd := m.pageSize * requestedPage
	if sliceEnd > totalSG {
		sliceEnd = totalSG
	}

	ret := &ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: m.sgs[sliceStart:sliceEnd],
	}

	if sliceEnd < totalSG {
		nextToken := strconv.Itoa(requestedPage + 1)
		ret.NextToken = &nextToken
	}

	return ret, nil
}

func TestListSecurityGroups(t *testing.T) {
	ctx := context.Background()

	noErrorFunc := func(err error) bool {
		return err == nil
	}

	const pageSize = 100
	t.Run("pagination", func(t *testing.T) {
		totalSecurityGroups := 203

		allSGs := make([]ec2Types.SecurityGroup, 0, totalSecurityGroups)
		for i := 0; i < totalSecurityGroups; i++ {
			allSGs = append(allSGs, ec2Types.SecurityGroup{
				GroupId:   aws.String(fmt.Sprintf("sg-%d", i)),
				GroupName: aws.String(fmt.Sprintf("MySG-%d", i)),
			})
		}

		mockListClient := &mockListSecurityGroupsClient{
			pageSize: pageSize,
			sgs:      allSGs,
		}

		// First page must return pageSize number of Security Groups
		resp, err := ListSecurityGroups(ctx, mockListClient, ListSecurityGroupsRequest{
			VPCID:     "vpc-123",
			NextToken: "",
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.SecurityGroups, pageSize)
		nextPageToken := resp.NextToken
		require.Equal(t, resp.SecurityGroups[0].ID, "sg-0")
		require.Equal(t, resp.SecurityGroups[0].Name, "MySG-0")

		// Second page must return pageSize number of Endpoints
		resp, err = ListSecurityGroups(ctx, mockListClient, ListSecurityGroupsRequest{
			VPCID:     "vpc-abc",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.SecurityGroups, pageSize)
		nextPageToken = resp.NextToken
		require.Equal(t, resp.SecurityGroups[0].ID, "sg-100")
		require.Equal(t, resp.SecurityGroups[0].Name, "MySG-100")

		// Third page must return only the remaining Endpoints and an empty nextToken
		resp, err = ListSecurityGroups(ctx, mockListClient, ListSecurityGroupsRequest{
			VPCID:     "vpc-abc",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Len(t, resp.SecurityGroups, 3)
		require.Equal(t, resp.SecurityGroups[0].ID, "sg-200")
		require.Equal(t, resp.SecurityGroups[0].Name, "MySG-200")
	})

	for _, tt := range []struct {
		name      string
		req       ListSecurityGroupsRequest
		mockSGs   []ec2Types.SecurityGroup
		errCheck  func(error) bool
		respCheck func(*testing.T, *ListSecurityGroupsResponse)
	}{
		{
			name: "valid for listing instances",
			req: ListSecurityGroupsRequest{
				VPCID:     "vpc-abcd",
				NextToken: "",
			},
			mockSGs: []ec2Types.SecurityGroup{{
				GroupId:   aws.String("sg-123"),
				GroupName: aws.String("MySG-123"),
			},
			},
			respCheck: func(t *testing.T, ldr *ListSecurityGroupsResponse) {
				require.Len(t, ldr.SecurityGroups, 1, "expected 1 SG, got %d", len(ldr.SecurityGroups))
				require.Empty(t, ldr.NextToken, "expected an empty NextToken")

				sg := SecurityGroup{
					Name: "MySG-123",
					ID:   "sg-123",
				}
				require.Empty(t, cmp.Diff(sg, ldr.SecurityGroups[0]))
			},
			errCheck: noErrorFunc,
		},
		{
			name:     "no vpc id",
			req:      ListSecurityGroupsRequest{},
			errCheck: trace.IsBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockListClient := &mockListSecurityGroupsClient{
				pageSize: pageSize,
				sgs:      tt.mockSGs,
			}
			resp, err := ListSecurityGroups(ctx, mockListClient, tt.req)
			require.True(t, tt.errCheck(err), "unexpected err: %v", err)
			if tt.respCheck != nil {
				tt.respCheck(t, resp)
			}
		})
	}
}
