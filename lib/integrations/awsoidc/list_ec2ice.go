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
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gravitational/trace"
)

// ListEC2ICERequest contains the required fields to list AWS EC2 Instance Connect Endpoints.
type ListEC2ICERequest struct {
	// Region is the region of the EICE.
	Region string

	// VPCIDs is a list of VPCs to filter EC2 Instance Connect Endpoints.
	VPCIDs []string

	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *ListEC2ICERequest) CheckAndSetDefaults() error {
	if req.Region == "" {
		return trace.BadParameter("region is required")
	}

	if len(req.VPCIDs) == 0 {
		return trace.BadParameter("at least one vpc id is required")
	}

	return nil
}

// EC2InstanceConnectEndpoint is the Teleport representation of an EC2 Instance Connect Endpoint
type EC2InstanceConnectEndpoint struct {
	// Name is the endpoint name.
	Name string `json:"name,omitempty"`

	// State is the endpoint state.
	// Known values:
	// create-in-progress | create-complete | create-failed | delete-in-progress | delete-complete | delete-failed
	State string `json:"state,omitempty"`

	// StateMessage contains a message describing the state of the EICE.
	// Can be empty.
	StateMessage string `json:"stateMessage,omitempty"`

	// DashboardLink is a URL to AWS Console where the user can see the EC2 Instance Connect Endpoint.
	DashboardLink string `json:"dashboardLink,omitempty"`

	// SubnetID is the subnet used by the endpoint.
	// Please note that the Endpoint should be able to reach any subnet within the VPC.
	SubnetID string `json:"subnetId,omitempty"`

	// VPCID is the VPC ID where the Endpoint is created.
	VPCID string `json:"vpcId,omitempty"`
}

// ListEC2ICEResponse contains a page of AWS EC2 Instances as Teleport Servers.
type ListEC2ICEResponse struct {
	// EC2ICEs contains the page of EC2 Instance Connect Endpoint.
	EC2ICEs []EC2InstanceConnectEndpoint `json:"ec2InstanceConnectEndpoints,omitempty"`

	// DashboardLink is the URL for AWS Web Console that lists all the Endpoints for the queries VPCs.
	DashboardLink string `json:"dashboardLink,omitempty"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken,omitempty"`
}

// ListEC2ICEClient describes the required methods to List EC2 Instances using a 3rd Party API.
type ListEC2ICEClient interface {
	// DescribeInstanceConnectEndpoints describes the specified EC2 Instance Connect Endpoints or all EC2 Instance
	// Connect Endpoints.
	DescribeInstanceConnectEndpoints(ctx context.Context, params *ec2.DescribeInstanceConnectEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceConnectEndpointsOutput, error)
}

type defaultListEC2ICEClient struct {
	*ec2.Client
}

// NewListEC2ICEClient creates a new ListEC2ICEClient using a AWSClientRequest.
func NewListEC2ICEClient(ctx context.Context, req *AWSClientRequest) (ListEC2ICEClient, error) {
	ec2Client, err := newEC2Client(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultListEC2ICEClient{
		Client: ec2Client,
	}, nil
}

// ListEC2ICE calls the following AWS API:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceConnectEndpoints.html
// It returns a list of EC2 Instance Connect Endpoints and an optional NextToken that can be used to fetch the next page
func ListEC2ICE(ctx context.Context, clt ListEC2ICEClient, req ListEC2ICERequest) (*ListEC2ICEResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	describeEC2ICE := &ec2.DescribeInstanceConnectEndpointsInput{
		Filters: []ec2Types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: req.VPCIDs,
		}},
	}
	if req.NextToken != "" {
		describeEC2ICE.NextToken = &req.NextToken
	}

	ec2ICEs, err := clt.DescribeInstanceConnectEndpoints(ctx, describeEC2ICE)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ret := &ListEC2ICEResponse{}

	if aws.ToString(ec2ICEs.NextToken) != "" {
		ret.NextToken = *ec2ICEs.NextToken
	}

	ret.DashboardLink = fmt.Sprintf(
		"https://%s.console.aws.amazon.com/vpcconsole/home?#Endpoints:v=3;vpcEndpointType=%s",
		req.Region,
		// We must use PathEscape here because it converts spaces into `%20`.
		// QueryEscape converts spaces into `+`, which, when loaded in AWS Console page, filters for `EC2+Instance+Connect+Endpoint`, instead of `EC2 Instance Connect Endpoint`
		url.PathEscape("EC2 Instance Connect Endpoint"),
	)

	ret.EC2ICEs = make([]EC2InstanceConnectEndpoint, 0, len(ec2ICEs.InstanceConnectEndpoints))
	for _, ice := range ec2ICEs.InstanceConnectEndpoints {
		name := aws.ToString(ice.InstanceConnectEndpointId)
		subnetID := aws.ToString(ice.SubnetId)
		vpcID := aws.ToString(ice.VpcId)
		state := ice.State
		stateMessage := aws.ToString(ice.StateMessage)

		idURLSafe := url.QueryEscape(name)
		dashboardLink := fmt.Sprintf("https://%s.console.aws.amazon.com/vpcconsole/home?#InstanceConnectEndpointDetails:instanceConnectEndpointId=%s",
			req.Region, idURLSafe,
		)

		ret.EC2ICEs = append(ret.EC2ICEs, EC2InstanceConnectEndpoint{
			Name:          name,
			SubnetID:      subnetID,
			VPCID:         vpcID,
			State:         string(state),
			StateMessage:  stateMessage,
			DashboardLink: dashboardLink,
		})
	}

	return ret, nil
}
