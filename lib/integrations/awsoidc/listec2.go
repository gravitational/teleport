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
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// awsInstanceStateName represents the state of the AWS EC2
	// instance - (pending | running | shutting-down | terminated | stopping | stopped )
	// https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-instances.html
	// Used for filtering instances.
	awsInstanceStateName = "instance-state-name"
)

var (
	// filterRunningEC2Instance is an EC2 DescribeInstances Filter to filter running instances.
	filterRunningEC2Instance = ec2Types.Filter{
		Name:   aws.String(awsInstanceStateName),
		Values: []string{string(ec2Types.InstanceStateNameRunning)},
	}
)

// ListEC2Request contains the required fields to list AWS EC2 Instances.
type ListEC2Request struct {
	// Integration is the AWS OIDC Integration name.
	// This is used to populate the Server resource.
	// When connecting to the Node, this is the integration that is going to be used.
	Integration string

	// Region is the AWS Region.
	Region string

	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *ListEC2Request) CheckAndSetDefaults() error {
	if req.Integration == "" {
		return trace.BadParameter("integration is required")
	}
	if req.Region == "" {
		return trace.BadParameter("region is required")
	}

	return nil
}

// ListEC2Response contains a page of AWS EC2 Instances as Teleport Servers.
type ListEC2Response struct {
	// Servers contains the page of Servers.
	Servers []types.Server

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string
}

// ListEC2Client describes the required methods to List EC2 Instances using a 3rd Party API.
type ListEC2Client interface {
	// DescribeInstances describes the specified instances or all instances.
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)

	// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type defaultListEC2Client struct {
	*ec2.Client
	stsClient *sts.Client
}

// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
func (d defaultListEC2Client) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return d.stsClient.GetCallerIdentity(ctx, params, optFns...)
}

// NewListEC2Client creates a new ListEC2Client using a AWSClientRequest.
func NewListEC2Client(ctx context.Context, req *AWSClientRequest) (ListEC2Client, error) {
	ec2Client, err := newEC2Client(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stsClient, err := newSTSClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultListEC2Client{
		Client:    ec2Client,
		stsClient: stsClient,
	}, nil
}

// ListEC2 calls the following AWS API:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
// It returns a list of EC2 Instances and an optional NextToken that can be used to fetch the next page
func ListEC2(ctx context.Context, clt ListEC2Client, req ListEC2Request) (*ListEC2Response, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	callerIdentity, err := clt.GetCallerIdentity(ctx, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accountID := aws.ToString(callerIdentity.Account)
	if accountID == "" {
		return nil, trace.BadParameter("failed to get AWS AccountID using GetCallerIdentity")
	}

	describeEC2Instances := &ec2.DescribeInstancesInput{
		Filters: []ec2Types.Filter{
			filterRunningEC2Instance,
		},
	}
	if req.NextToken != "" {
		describeEC2Instances.NextToken = &req.NextToken
	}

	ec2Instances, err := clt.DescribeInstances(ctx, describeEC2Instances)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ret := &ListEC2Response{}

	if aws.ToString(ec2Instances.NextToken) != "" {
		ret.NextToken = *ec2Instances.NextToken
	}

	ret.Servers = make([]types.Server, 0, len(ec2Instances.Reservations))
	for _, reservation := range ec2Instances.Reservations {
		for _, instance := range reservation.Instances {
			awsInfo := &types.AWSInfo{
				AccountID:   accountID,
				Region:      req.Region,
				Integration: req.Integration,
			}

			server, err := services.NewAWSNodeFromEC2Instance(instance, awsInfo)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			ret.Servers = append(ret.Servers, server)
		}
	}

	return ret, nil
}
