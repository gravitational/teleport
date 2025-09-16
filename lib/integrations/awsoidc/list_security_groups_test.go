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
		require.Equal(t, "sg-0", resp.SecurityGroups[0].ID)
		require.Equal(t, "MySG-0", resp.SecurityGroups[0].Name)

		// Second page must return pageSize number of Endpoints
		resp, err = ListSecurityGroups(ctx, mockListClient, ListSecurityGroupsRequest{
			VPCID:     "vpc-abc",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.SecurityGroups, pageSize)
		nextPageToken = resp.NextToken
		require.Equal(t, "sg-100", resp.SecurityGroups[0].ID)
		require.Equal(t, "MySG-100", resp.SecurityGroups[0].Name)

		// Third page must return only the remaining Endpoints and an empty nextToken
		resp, err = ListSecurityGroups(ctx, mockListClient, ListSecurityGroupsRequest{
			VPCID:     "vpc-abc",
			NextToken: nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Len(t, resp.SecurityGroups, 3)
		require.Equal(t, "sg-200", resp.SecurityGroups[0].ID)
		require.Equal(t, "MySG-200", resp.SecurityGroups[0].Name)
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
					Name:          "MySG-123",
					ID:            "sg-123",
					InboundRules:  []SecurityGroupRule{},
					OutboundRules: []SecurityGroupRule{},
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

func TestConvertSecurityGroup(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    []ec2Types.SecurityGroup
		expected []SecurityGroup
	}{
		{
			name: "no rules",
			input: []ec2Types.SecurityGroup{{
				GroupId:   aws.String("sg-123"),
				GroupName: aws.String("my group"),
			}},
			expected: []SecurityGroup{{
				ID:            "sg-123",
				Name:          "my group",
				InboundRules:  []SecurityGroupRule{},
				OutboundRules: []SecurityGroupRule{},
			}},
		},
		{
			name: "inbound rule allows SSH, outbound allows everything",
			input: []ec2Types.SecurityGroup{
				{
					GroupId:     aws.String("sg-123"),
					GroupName:   aws.String("my group"),
					Description: aws.String("my first vpc"),
					IpPermissions: []ec2Types.IpPermission{{
						FromPort:   aws.Int32(22),
						ToPort:     aws.Int32(22),
						IpProtocol: aws.String("tcp"),
						IpRanges: []ec2Types.IpRange{{
							CidrIp:      aws.String("0.0.0.0/0"),
							Description: aws.String("Everything"),
						}},
					}},
					IpPermissionsEgress: []ec2Types.IpPermission{{
						FromPort:   aws.Int32(0),
						ToPort:     aws.Int32(0),
						IpProtocol: aws.String("-1"),
						IpRanges: []ec2Types.IpRange{{
							CidrIp:      aws.String("0.0.0.0/0"),
							Description: aws.String("Everything"),
						}},
					}},
				},
			},
			expected: []SecurityGroup{
				{
					ID:          "sg-123",
					Name:        "my group",
					Description: "my first vpc",
					OutboundRules: []SecurityGroupRule{{
						IPProtocol: "all",
						FromPort:   0,
						ToPort:     0,
						CIDRs: []CIDR{{
							CIDR:        "0.0.0.0/0",
							Description: "Everything",
						}},
					}},
					InboundRules: []SecurityGroupRule{{
						IPProtocol: "tcp",
						FromPort:   22,
						ToPort:     22,
						CIDRs: []CIDR{{
							CIDR:        "0.0.0.0/0",
							Description: "Everything",
						}},
					}},
				},
			},
		},
		{
			name: "multiple inbound and outbound rules",
			input: []ec2Types.SecurityGroup{
				{
					GroupId:   aws.String("sg-123"),
					GroupName: aws.String("my group"),
					IpPermissions: []ec2Types.IpPermission{
						{
							FromPort:   aws.Int32(3000),
							ToPort:     aws.Int32(4000),
							IpProtocol: aws.String("tcp"),
							IpRanges:   []ec2Types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
						},
						{
							FromPort:   aws.Int32(443),
							ToPort:     aws.Int32(443),
							IpProtocol: aws.String("tcp"),
							IpRanges:   []ec2Types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
						},
						{
							FromPort:   aws.Int32(80),
							ToPort:     aws.Int32(80),
							IpProtocol: aws.String("tcp"),
							IpRanges:   []ec2Types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
						},
						{
							FromPort:   aws.Int32(22),
							ToPort:     aws.Int32(22),
							IpProtocol: aws.String("tcp"),
							IpRanges:   []ec2Types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
							UserIdGroupPairs: []ec2Types.UserIdGroupPair{{
								GroupId:     aws.String("sg-123"),
								Description: aws.String("allowed from another sg"),
							}},
						},
					},
					IpPermissionsEgress: []ec2Types.IpPermission{
						{
							FromPort:   aws.Int32(443),
							ToPort:     aws.Int32(443),
							IpProtocol: aws.String("tcp"),
							IpRanges:   []ec2Types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
						},
						{
							FromPort:   aws.Int32(3080),
							ToPort:     aws.Int32(3080),
							IpProtocol: aws.String("tcp"),
							IpRanges: []ec2Types.IpRange{{
								CidrIp:      aws.String("0.0.0.0/0"),
								Description: aws.String("Everything"),
							}},
							UserIdGroupPairs: []ec2Types.UserIdGroupPair{{
								GroupId:     aws.String("sg-456"),
								Description: aws.String("allowed to another sg"),
							}},
						},
					},
				},
			},
			expected: []SecurityGroup{
				{
					ID:   "sg-123",
					Name: "my group",
					InboundRules: []SecurityGroupRule{
						{
							IPProtocol: "tcp",
							FromPort:   3000,
							ToPort:     4000,
							CIDRs:      []CIDR{{CIDR: "0.0.0.0/0"}},
						},
						{
							IPProtocol: "tcp",
							FromPort:   443,
							ToPort:     443,
							CIDRs:      []CIDR{{CIDR: "0.0.0.0/0"}},
						},
						{
							IPProtocol: "tcp",
							FromPort:   80,
							ToPort:     80,
							CIDRs:      []CIDR{{CIDR: "0.0.0.0/0"}},
						},
						{
							IPProtocol: "tcp",
							FromPort:   22,
							ToPort:     22,
							CIDRs:      []CIDR{{CIDR: "0.0.0.0/0"}},
							Groups:     []GroupIDRule{{GroupId: "sg-123", Description: "allowed from another sg"}},
						},
					},
					OutboundRules: []SecurityGroupRule{
						{
							IPProtocol: "tcp",
							FromPort:   443,
							ToPort:     443,
							CIDRs: []CIDR{{
								CIDR: "0.0.0.0/0",
							}},
						},
						{
							IPProtocol: "tcp",
							FromPort:   3080,
							ToPort:     3080,
							CIDRs: []CIDR{{
								CIDR:        "0.0.0.0/0",
								Description: "Everything",
							}},
							Groups: []GroupIDRule{{GroupId: "sg-456", Description: "allowed to another sg"}},
						},
					},
				},
			},
		},
		{
			name: "multiple CIDRs",
			input: []ec2Types.SecurityGroup{
				{
					GroupId:   aws.String("sg-123"),
					GroupName: aws.String("my group"),
					IpPermissions: []ec2Types.IpPermission{
						{
							FromPort:   aws.Int32(3000),
							ToPort:     aws.Int32(4000),
							IpProtocol: aws.String("tcp"),
							IpRanges: []ec2Types.IpRange{
								{
									CidrIp:      aws.String("192.168.1.0/24"),
									Description: aws.String("Subnet Mask 255.255.255.0"),
								},
								{
									CidrIp:      aws.String("10.0.0.0/16"),
									Description: aws.String("Subnet Mask 255.255.0.0"),
								},
							},
						},
					},
				},
			},
			expected: []SecurityGroup{
				{
					ID:   "sg-123",
					Name: "my group",
					InboundRules: []SecurityGroupRule{
						{
							IPProtocol: "tcp",
							FromPort:   3000,
							ToPort:     4000,
							CIDRs: []CIDR{
								{
									CIDR:        "192.168.1.0/24",
									Description: "Subnet Mask 255.255.255.0",
								},
								{
									CIDR:        "10.0.0.0/16",
									Description: "Subnet Mask 255.255.0.0",
								},
							},
						},
					},
					OutboundRules: []SecurityGroupRule{},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, convertAWSSecurityGroups(tt.input))
		})
	}
}
