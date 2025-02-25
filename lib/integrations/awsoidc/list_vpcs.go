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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gravitational/trace"
)

// ListVPCsRequest contains the required fields to list AWS VPCs.
type ListVPCsRequest struct {
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

// VPC is the Teleport representation of an AWS VPC.
type VPC struct {
	// Name is the VPC name.
	// This is just a friendly name and should not be used for further API calls.
	// It can be empty if the VPC was not given a "Name" tag.
	Name string `json:"name"`

	// ID is the VPC ID, for example "vpc-0ee975135dEXAMPLE".
	// This is the value that should be used when doing further API calls.
	ID string `json:"id"`
}

// ListVPCsResponse contains a page of VPCs.
type ListVPCsResponse struct {
	// VPCs contains the page of VPCs.
	VPCs []VPC `json:"vpcs"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken"`
}

// ListVPCsClient describes the required methods to list AWS VPCs.
type ListVPCsClient interface {
	// DescribeVpcs describes VPCs.
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
}

type defaultListVPCsClient struct {
	*ec2.Client
}

// NewListVPCsClient creates a new ListVPCsClient using an AWSClientRequest.
func NewListVPCsClient(ctx context.Context, req *AWSClientRequest) (ListVPCsClient, error) {
	ec2Client, err := newEC2Client(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultListVPCsClient{
		Client: ec2Client,
	}, nil
}

// ListVPCs calls the following AWS API:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVpcs.html
// It returns a list of VPCs and an optional NextToken that can be used to fetch the next page.
func ListVPCs(ctx context.Context, clt ListVPCsClient, req ListVPCsRequest) (*ListVPCsResponse, error) {
	describeVPCsInput := &ec2.DescribeVpcsInput{MaxResults: aws.Int32(100)}
	if req.NextToken != "" {
		describeVPCsInput.NextToken = &req.NextToken
	}

	resp, err := clt.DescribeVpcs(ctx, describeVPCsInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ListVPCsResponse{
		NextToken: aws.ToString(resp.NextToken),
		VPCs:      convertAWSVPCs(resp.Vpcs),
	}, nil
}

func convertAWSVPCs(vpcs []ec2Types.Vpc) []VPC {
	ret := make([]VPC, 0, len(vpcs))
	for _, v := range vpcs {
		ret = append(ret, VPC{
			Name: nameFromEC2Tags(v.Tags),
			ID:   aws.ToString(v.VpcId),
		})
	}
	return ret
}

// nameFromEC2Tags is a helper to find the display name of an ec2 resource based
// on the "Name" tag, if it exists.
// Returns an empty string if there is no "Name" tag.
func nameFromEC2Tags(tags []ec2Types.Tag) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "Name" {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}
