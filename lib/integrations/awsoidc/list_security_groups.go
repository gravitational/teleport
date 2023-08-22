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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gravitational/trace"
)

// ListSecurityGroupsRequest contains the required fields to list VPC Security Groups.
type ListSecurityGroupsRequest struct {
	// VPCID is the VPC to filter Security Groups.
	VPCID string

	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *ListSecurityGroupsRequest) CheckAndSetDefaults() error {
	if req.VPCID == "" {
		return trace.BadParameter("vpc id is required")
	}

	return nil
}

// SecurityGroup is the Teleport representation of an EC2 Instance Connect Endpoint
type SecurityGroup struct {
	// Name is the Security Group name.
	// This is just a friendly name and should not be used for further API calls
	Name string

	// SecurityGroupID is the security group ID.
	// This is the value that should be used when doing further API calls.
	SecurityGroupID string
}

// ListSecurityGroupsResponse contains a page of SecurityGroups.
type ListSecurityGroupsResponse struct {
	// SecurityGroups contains the page of VPC Security Groups.
	SecurityGroups []SecurityGroup

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string
}

// ListSecurityGroupsClient describes the required methods to List Security Groups a 3rd Party API.
type ListSecurityGroupsClient interface {
	// DescribeSecurityGroups describes the specified security groups or all of your security groups.
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

type defaultListSecurityGroupsClient struct {
	*ec2.Client
}

// NewListSecurityGroupsClient creates a new ListSecurityGroupsClient using a AWSClientRequest.
func NewListSecurityGroupsClient(ctx context.Context, req *AWSClientRequest) (ListSecurityGroupsClient, error) {
	ec2Client, err := newEC2Client(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultListSecurityGroupsClient{
		Client: ec2Client,
	}, nil
}

// ListSecurityGroups calls the following AWS API:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html
// It returns a list of VPC Security Groups and an optional NextToken that can be used to fetch the next page
func ListSecurityGroups(ctx context.Context, clt ListSecurityGroupsClient, req ListSecurityGroupsRequest) (*ListSecurityGroupsResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	describeSecurityGroups := &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2Types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{req.VPCID},
		}},
	}
	if req.NextToken != "" {
		describeSecurityGroups.NextToken = &req.NextToken
	}

	securityGroupsResp, err := clt.DescribeSecurityGroups(ctx, describeSecurityGroups)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ret := &ListSecurityGroupsResponse{}

	if aws.ToString(securityGroupsResp.NextToken) != "" {
		ret.NextToken = *securityGroupsResp.NextToken
	}

	ret.SecurityGroups = make([]SecurityGroup, 0, len(securityGroupsResp.SecurityGroups))
	for _, sg := range securityGroupsResp.SecurityGroups {
		sgName := aws.ToString(sg.GroupName)
		sgID := aws.ToString(sg.GroupId)

		ret.SecurityGroups = append(ret.SecurityGroups, SecurityGroup{
			Name:            sgName,
			SecurityGroupID: sgID,
		})
	}

	return ret, nil
}
