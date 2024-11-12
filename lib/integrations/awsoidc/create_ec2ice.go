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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
)

// CreateEC2ICERequest contains the required fields to create an AWS EC2 Instance Connect Endpoint.
type CreateEC2ICERequest struct {
	// Cluster is the Teleport Cluster Name.
	// Used to tag resources created in AWS.
	Cluster string

	// IntegrationName is the integration name.
	// Used to tag resources created in AWS.
	IntegrationName string

	// Endpoints is a list of EC2 Instance Connect Endpoints to be created.
	Endpoints []EC2ICEEndpoint

	// ResourceCreationTags is used to add tags when creating resources in AWS.
	// Defaults to:
	// - teleport.dev/cluster: <cluster>
	// - teleport.dev/origin: aws-oidc-integration
	// - teleport.dev/integration: <integrationName>
	ResourceCreationTags tags.AWSTags
}

// EC2ICEEndpoint contains the information for a single Endpoint to be created.
type EC2ICEEndpoint struct {
	// Name is the endpoint name.
	Name string

	// SubnetID is the Subnet where the Endpoint will be created.
	SubnetID string

	// SecurityGroupIDs is a list of SecurityGroups to assign to the Endpoint.
	// If not specified, the Endpoint will receive the default SG for the Subnet's VPC.
	SecurityGroupIDs []string
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *CreateEC2ICERequest) CheckAndSetDefaults() error {
	if req.Cluster == "" {
		return trace.BadParameter("cluster is required")
	}

	if req.IntegrationName == "" {
		return trace.BadParameter("integration is required")
	}

	if len(req.Endpoints) == 0 {
		return trace.BadParameter("endpoints is required")
	}

	for i, endpoint := range req.Endpoints {
		if endpoint.SubnetID == "" {
			return trace.BadParameter("missing Subnet in endpoint (index %d)", i)
		}
	}

	if len(req.ResourceCreationTags) == 0 {
		req.ResourceCreationTags = tags.DefaultResourceCreationTags(req.Cluster, req.IntegrationName)
	}

	return nil
}

// CreateEC2ICEResponse contains the newly created EC2 Instance Connect Endpoint name.
type CreateEC2ICEResponse struct {
	// Name is the Endpoint ID.
	Name string

	// CreatedEndpoints contains the name of created endpoints and their Subnet.
	CreatedEndpoints []EC2ICEEndpoint
}

// CreateEC2ICE describes the required methods to List EC2 Instances using a 3rd Party API.
type CreateEC2ICEClient interface {
	// CreateInstanceConnectEndpoint creates an EC2 Instance Connect Endpoint. An EC2 Instance Connect Endpoint
	// allows you to connect to an instance, without requiring the instance to have a
	// public IPv4 address. For more information, see Connect to your instances
	// without requiring a public IPv4 address using EC2 Instance Connect Endpoint (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Connect-using-EC2-Instance-Connect-Endpoint.html)
	// in the Amazon EC2 User Guide.
	CreateInstanceConnectEndpoint(ctx context.Context, params *ec2.CreateInstanceConnectEndpointInput, optFns ...func(*ec2.Options)) (*ec2.CreateInstanceConnectEndpointOutput, error)
}

type defaultCreateEC2ICEClient struct {
	*ec2.Client
}

// NewCreateEC2ICEClient creates a new CreateEC2ICEClient using a AWSClientRequest.
func NewCreateEC2ICEClient(ctx context.Context, req *AWSClientRequest) (CreateEC2ICEClient, error) {
	ec2Client, err := newEC2Client(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultCreateEC2ICEClient{
		Client: ec2Client,
	}, nil
}

// CreateEC2ICE calls the following AWS API:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateInstanceConnectEndpoint.html
// It creates an EC2 Instance Connect Endpoint using the provided Subnet and Security Group IDs.
func CreateEC2ICE(ctx context.Context, clt CreateEC2ICEClient, req CreateEC2ICERequest) (*CreateEC2ICEResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	createdEndpoints := make([]EC2ICEEndpoint, 0, len(req.Endpoints))
	endpointNames := make([]string, 0, len(req.Endpoints))

	for _, endpoint := range req.Endpoints {
		createEC2ICEInput := &ec2.CreateInstanceConnectEndpointInput{
			SubnetId:         &endpoint.SubnetID,
			SecurityGroupIds: endpoint.SecurityGroupIDs,
			TagSpecifications: []ec2types.TagSpecification{{
				ResourceType: ec2types.ResourceTypeInstanceConnectEndpoint,
				Tags:         req.ResourceCreationTags.ToEC2Tags(),
			}},
		}

		ec2ICEndpoint, err := clt.CreateInstanceConnectEndpoint(ctx, createEC2ICEInput)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Sentinel value that indicates that the API returned a nil ec2ICEndpoint.InstanceConnectEndpoint.
		// Very unlikely to happen.
		endpointName := "unknown"
		if ec2ICEndpoint.InstanceConnectEndpoint != nil {
			endpointName = aws.ToString(ec2ICEndpoint.InstanceConnectEndpoint.InstanceConnectEndpointId)
		}
		endpointNames = append(endpointNames, endpointName)

		createdEndpoints = append(createdEndpoints, EC2ICEEndpoint{
			Name:             endpointName,
			SubnetID:         endpoint.SubnetID,
			SecurityGroupIDs: endpoint.SecurityGroupIDs,
		})
	}

	return &CreateEC2ICEResponse{
		Name:             strings.Join(endpointNames, ","),
		CreatedEndpoints: createdEndpoints,
	}, nil
}
