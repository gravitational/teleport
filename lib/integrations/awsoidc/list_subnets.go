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

// ListSubnetsRequest contains the required fields to list AWS VPC subnets.
type ListSubnetsRequest struct {
	// VPCID is the VPC to filter subnets.
	VPCID string

	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *ListSubnetsRequest) CheckAndSetDefaults() error {
	if req.VPCID == "" {
		return trace.BadParameter("vpc id is required")
	}

	return nil
}

// Subnet is the Teleport representation of an AWS VPC subnet.
type Subnet struct {
	// Name is the subnet name.
	// This is just a friendly name and should not be used for further API calls.
	// It can be empty if the subnet was not given a "Name" tag.
	Name string `json:"name"`

	// ID is the subnet ID, for example "subnet-0b3ca383195ad2cc7".
	// This is the value that should be used when doing further API calls.
	ID string `json:"id"`

	// AvailabilityZone is the AWS availability zone of the subnet, for example
	// "us-west-1a".
	AvailabilityZone string `json:"availability_zone"`
}

// ListSubnetsResponse contains a page of subnets.
type ListSubnetsResponse struct {
	// Subnets contains the page of VPC subnets.
	Subnets []Subnet `json:"subnets"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken"`
}

// ListSubnetsClient describes the required methods to list AWS VPC subnets.
type ListSubnetsClient interface {
	// DescribeSubnets describes the specified subnets.
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
}

type defaultListSubnetsClient struct {
	*ec2.Client
}

// NewListSubnetsClient creates a new ListSubnetsClient using an AWSClientRequest.
func NewListSubnetsClient(ctx context.Context, req *AWSClientRequest) (ListSubnetsClient, error) {
	ec2Client, err := newEC2Client(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultListSubnetsClient{
		Client: ec2Client,
	}, nil
}

// ListSubnets calls the following AWS API:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html
// It returns a list of VPC subnets and an optional NextToken that can be used to fetch the next page.
func ListSubnets(ctx context.Context, clt ListSubnetsClient, req ListSubnetsRequest) (*ListSubnetsResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	describeSubnetsInput := &ec2.DescribeSubnetsInput{
		Filters: []ec2Types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{req.VPCID},
		}},
	}
	if req.NextToken != "" {
		describeSubnetsInput.NextToken = &req.NextToken
	}

	resp, err := clt.DescribeSubnets(ctx, describeSubnetsInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ListSubnetsResponse{
		NextToken: aws.ToString(resp.NextToken),
		Subnets:   convertAWSSubnets(resp.Subnets),
	}, nil
}

func convertAWSSubnets(subnets []ec2Types.Subnet) []Subnet {
	ret := make([]Subnet, 0, len(subnets))
	for _, s := range subnets {
		ret = append(ret, Subnet{
			Name:             nameFromEC2Tags(s.Tags),
			ID:               aws.ToString(s.SubnetId),
			AvailabilityZone: aws.ToString(s.AvailabilityZone),
		})
	}

	return ret
}
