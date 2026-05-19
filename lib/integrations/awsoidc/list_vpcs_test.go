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

var _ ListVPCsClient = (*mockListVPCsClient)(nil)

type mockListVPCsClient struct {
	pageSize int
	vpcs     []ec2Types.Vpc
}

// Returns information about AWS VPCs.
// This API supports pagination.
func (m mockListVPCsClient) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	requestedPage := 1

	totalVPCs := len(m.vpcs)

	if params.NextToken != nil {
		currentMarker, err := strconv.Atoi(*params.NextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := m.pageSize * (requestedPage - 1)
	sliceEnd := m.pageSize * requestedPage
	if sliceEnd > totalVPCs {
		sliceEnd = totalVPCs
	}

	ret := &ec2.DescribeVpcsOutput{
		Vpcs: m.vpcs[sliceStart:sliceEnd],
	}

	if sliceEnd < totalVPCs {
		nextToken := strconv.Itoa(requestedPage + 1)
		ret.NextToken = &nextToken
	}

	return ret, nil
}

func TestListVPCs(t *testing.T) {
	ctx := context.Background()

	noErrorFunc := func(err error) bool {
		return err == nil
	}

	const pageSize = 100
	t.Run("pagination", func(t *testing.T) {
		totalVPCs := 203

		VPCs := make([]ec2Types.Vpc, 0, totalVPCs)
		for i := 0; i < totalVPCs; i++ {
			VPCs = append(VPCs, ec2Types.Vpc{
				VpcId: aws.String(fmt.Sprintf("VPC-%d", i)),
				Tags:  makeNameTags(fmt.Sprintf("MyVPC-%d", i)),
			})
		}

		mockListClient := &mockListVPCsClient{
			pageSize: pageSize,
			vpcs:     VPCs,
		}

		// First page must return pageSize number of VPCs
		resp, err := ListVPCs(ctx, mockListClient, ListVPCsRequest{
			NextToken: "",
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.VPCs, pageSize)
		nextPageToken := resp.NextToken
		require.Equal(t, "VPC-0", resp.VPCs[0].ID)
		require.Equal(t, "MyVPC-0", resp.VPCs[0].Name)

		// Second page must return pageSize number of Endpoints
		resp, err = ListVPCs(ctx, mockListClient, ListVPCsRequest{
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.VPCs, pageSize)
		nextPageToken = resp.NextToken
		require.Equal(t, "VPC-100", resp.VPCs[0].ID)
		require.Equal(t, "MyVPC-100", resp.VPCs[0].Name)

		// Third page must return only the remaining Endpoints and an empty nextToken
		resp, err = ListVPCs(ctx, mockListClient, ListVPCsRequest{
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Len(t, resp.VPCs, 3)
		require.Equal(t, "VPC-200", resp.VPCs[0].ID)
		require.Equal(t, "MyVPC-200", resp.VPCs[0].Name)
	})

	for _, tt := range []struct {
		name      string
		req       ListVPCsRequest
		mockVPCs  []ec2Types.Vpc
		errCheck  func(error) bool
		respCheck func(*testing.T, *ListVPCsResponse)
	}{
		{
			name: "valid for listing VPCs",
			req: ListVPCsRequest{
				NextToken: "",
			},
			mockVPCs: []ec2Types.Vpc{{
				VpcId: aws.String("VPC-123"),
				Tags:  makeNameTags("MyVPC-123"),
			}},
			respCheck: func(t *testing.T, ldr *ListVPCsResponse) {
				require.Len(t, ldr.VPCs, 1)
				require.Empty(t, ldr.NextToken, "there is only 1 page of VPCs")

				want := VPC{
					ID:   "VPC-123",
					Name: "MyVPC-123",
				}
				require.Empty(t, cmp.Diff(want, ldr.VPCs[0]))
			},
			errCheck: noErrorFunc,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockListClient := &mockListVPCsClient{
				pageSize: pageSize,
				vpcs:     tt.mockVPCs,
			}
			resp, err := ListVPCs(ctx, mockListClient, tt.req)
			require.True(t, tt.errCheck(err), "unexpected err: %v", err)
			if tt.respCheck != nil {
				tt.respCheck(t, resp)
			}
		})
	}
}

func TestConvertVPC(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    []ec2Types.Vpc
		expected []VPC
	}{
		{
			name: "no name tag",
			input: []ec2Types.Vpc{{
				VpcId: aws.String("VPC-abc"),
				Tags: []ec2Types.Tag{
					{Key: aws.String("foo"), Value: aws.String("bar")},
				},
			}},
			expected: []VPC{{
				Name: "",
				ID:   "VPC-abc",
			}},
		},
		{
			name: "with name tag",
			input: []ec2Types.Vpc{{
				VpcId: aws.String("VPC-abc"),
				Tags: []ec2Types.Tag{
					{Key: aws.String("foo"), Value: aws.String("bar")},
					{Key: aws.String("Name"), Value: aws.String("llama")},
					{Key: aws.String("baz"), Value: aws.String("qux")},
				},
			}},
			expected: []VPC{{
				Name: "llama",
				ID:   "VPC-abc",
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, convertAWSVPCs(tt.input))
		})
	}
}
