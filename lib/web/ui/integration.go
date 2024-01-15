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

package ui

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
)

// IntegrationAWSOIDCSpec contain the specific fields for the `aws-oidc` subkind integration.
type IntegrationAWSOIDCSpec struct {
	// RoleARN is the role associated with the integration when SubKind is `aws-oidc`
	RoleARN string `json:"roleArn,omitempty"`
}

// Integration describes Integration fields
type Integration struct {
	// Name is the Integration name.
	Name string `json:"name,omitempty"`
	// SubKind is the Integration SubKind.
	SubKind string `json:"subKind,omitempty"`
	// AWSOIDC contains the fields for `aws-oidc` subkind integration.
	AWSOIDC *IntegrationAWSOIDCSpec `json:"awsoidc,omitempty"`
}

// CheckAndSetDefaults for the create request.
// Name and SubKind is required.
func (r *Integration) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing integration name")
	}

	if r.SubKind == "" {
		return trace.BadParameter("missing subKind")
	}

	if r.AWSOIDC != nil && r.AWSOIDC.RoleARN == "" {
		return trace.BadParameter("missing awsoidc.roleArn field")
	}

	return nil
}

// UpdateIntegrationRequest is a request to update an Integration
type UpdateIntegrationRequest struct {
	// AWSOIDC contains the fields for `aws-oidc` subkind integration.
	AWSOIDC *IntegrationAWSOIDCSpec `json:"awsoidc,omitempty"`
}

// CheckAndSetDefaults checks if the provided values are valid.
func (r *UpdateIntegrationRequest) CheckAndSetDefaults() error {
	if r.AWSOIDC != nil && r.AWSOIDC.RoleARN == "" {
		return trace.BadParameter("missing awsoidc.roleArn field")
	}

	return nil
}

// IntegrationsListResponse contains a list of Integrations.
// In case of exceeding the pagination limit (either via query param `limit` or the default 1000)
// a `nextToken` is provided and should be used to obtain the next page (as a query param `startKey`)
type IntegrationsListResponse struct {
	// Items is a list of resources retrieved.
	Items []Integration `json:"items"`
	// NextKey is the position to resume listing events.
	NextKey string `json:"nextKey"`
}

// MakeIntegrations creates a UI list of Integrations.
func MakeIntegrations(igs []types.Integration) []Integration {
	uiList := make([]Integration, 0, len(igs))

	for _, ig := range igs {
		uiList = append(uiList, MakeIntegration(ig))
	}

	return uiList
}

// MakeIntegration creates a UI Integration representation.
func MakeIntegration(ig types.Integration) Integration {
	return Integration{
		Name:    ig.GetName(),
		SubKind: ig.GetSubKind(),
		AWSOIDC: &IntegrationAWSOIDCSpec{
			RoleARN: ig.GetAWSOIDCIntegrationSpec().RoleARN,
		},
	}
}

// AWSOIDCListDatabasesRequest is a request to ListDatabases using the AWS OIDC Integration.
type AWSOIDCListDatabasesRequest struct {
	// RDSType is either `instance` or `cluster`.
	RDSType string `json:"rdsType"`
	// Engines filters the returned Databases based on their engine.
	// Eg, mysql, postgres, mariadb, aurora, aurora-mysql, aurora-postgresql
	Engines []string `json:"engines"`
	// Region is the AWS Region.
	Region string `json:"region"`
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string `json:"nextToken"`
}

// AWSOIDCListDatabasesResponse contains a list of databases and a next token if more pages are available.
type AWSOIDCListDatabasesResponse struct {
	// Databases contains the page of Databases
	Databases []Database `json:"databases"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken,omitempty"`
}

// AWSOIDCDeployServiceRequest contains the required fields to perform a DeployService request.
type AWSOIDCDeployServiceRequest struct {
	// Region is the AWS Region for the Service.
	Region string `json:"region"`

	// AccountID is the AWS Account ID.
	// Optional. sts.GetCallerIdentity is used if the value is not provided.
	AccountID string `json:"accountId"`

	// SubnetIDs associated with the Service.
	// If deploying a Database Service, you should use the SubnetIDs returned by the List Database API call.
	SubnetIDs []string `json:"subnetIds"`

	// SecurityGroups to apply to the service's network configuration.
	// If empty, the default security group for the VPC is going to be used.
	SecurityGroups []string `json:"securityGroups"`

	// ClusterName is the ECS Cluster to be used.
	// Optional.
	// Defaults to <teleport-cluster-name>-teleport, eg. acme-teleport
	ClusterName *string `json:"clusterName"`

	// ServiceName is the ECS Service that should be used.
	// Optional.
	// Defaults to <teleport-cluster-name>-teleport-service, eg acme-teleport-service
	ServiceName *string `json:"serviceName"`

	// TaskName is the ECS Task Definition family name.
	// Optional.
	// Defaults to <teleport-cluster-name>-teleport-<deployment-mode>, eg acme-teleport-database-service
	TaskName *string `json:"taskName"`

	// TaskRoleARN is the AWS Role's ARN used within the Task execution.
	// Ensure the AWS Client's Role has `iam:PassRole` for this Role's ARN.
	// This can be either the ARN or the short name of the AWS Role.
	TaskRoleARN string `json:"taskRoleArn"`

	// DeploymentMode is the deployment configuration for the service.
	// This indicates what set of services should be deployed.
	DeploymentMode string `json:"deploymentMode"`

	// DatabaseAgentMatcherLabels are the labels to be used when deploying a Database Service.
	// Those are the resource labels that the Service will monitor and proxy connections to.
	DatabaseAgentMatcherLabels []Label `json:"databaseAgentMatcherLabels"`
}

// AWSOIDCDeployServiceResponse contains the resources that were used to deploy a Teleport Service.
type AWSOIDCDeployServiceResponse struct {
	// ClusterARN is the Amazon ECS Cluster ARN where the task was started.
	ClusterARN string `json:"clusterArn"`

	// ServiceARN is the Amazon ECS Cluster Service ARN created to run the task.
	ServiceARN string `json:"serviceArn"`

	// TaskDefinitionARN is the Amazon ECS Task Definition ARN created to run the Service.
	TaskDefinitionARN string `json:"taskDefinitionArn"`

	// ServiceDashboardURL is a link to the service's Dashboard URL in Amazon Console.
	ServiceDashboardURL string `json:"serviceDashboardUrl"`
}

// AWSOIDCDeployDatabaseServiceRequest contains the required fields to perform a DeployService request.
// Each deployed DatabaseService will be proxying the resources that match the following labels:
// -region: <Region>
// -account-id: <AccountID>
// -vpc-id: <Deployments[].VPCID>
type AWSOIDCDeployDatabaseServiceRequest struct {
	// Region is the AWS Region for the Service.
	Region string `json:"region"`

	// TaskRoleARN is the AWS Role's ARN used within the Task execution.
	// Ensure the AWS Client's Role has `iam:PassRole` for this Role's ARN.
	// This can be either the ARN or the short name of the AWS Role.
	TaskRoleARN string `json:"taskRoleArn"`

	// Deployments is a list of Services to be deployed.
	// If the target deployment already exists, the deployment is skipped.
	Deployments []DeployDatabaseServiceDeployment `json:"deployments"`
}

// DeployDatabaseServiceDeployment identifies the required fields to deploy a DatabaseService.
type DeployDatabaseServiceDeployment struct {
	// VPCID is the VPCID where the service is going to be deployed.
	VPCID string `json:"vpcId"`

	// SubnetIDs are the subnets for the network configuration.
	// They must belong to the VPCID above.
	SubnetIDs []string `json:"subnetIds"`

	// SecurityGroups are the SecurityGroup IDs to associate with this particular deployment.
	// If empty, the default security group for the VPC is going to be used.
	SecurityGroups []string `json:"securityGroups"`
}

// AWSOIDCDeployServiceDatabaseResponse contains links to the ECS Cluster Dashboard where the current status for each Service is displayed.
type AWSOIDCDeployDatabaseServiceResponse struct {
	// ClusterARN is the Amazon ECS Cluster ARN where the Services were started.
	ClusterARN string `json:"clusterArn"`

	// ClusterDashboardURL is the URL for the Cluster Dashbord.
	// Users can open this link and see which Services are running.
	ClusterDashboardURL string `json:"clusterDashboardUrl"`
}

// AWSOIDCListEKSClustersRequest is a request to ListEKSClusters using the AWS OIDC Integration.
type AWSOIDCListEKSClustersRequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string `json:"nextToken"`
}

// AWSOIDCListEKSClustersResponse contains a list of clusters and a next token if more pages are available.
type AWSOIDCListEKSClustersResponse struct {
	// Clusters contains the page with list of EKSCluster
	Clusters []EKSCluster `json:"clusters"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken,omitempty"`
}

// AWSOIDCListEC2Request is a request to ListEC2s using the AWS OIDC Integration.
type AWSOIDCListEC2Request struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string `json:"nextToken"`
}

// AWSOIDCListEC2Response contains a list of Servers and a next token if more pages are available.
type AWSOIDCListEC2Response struct {
	// Servers contains the page of Servers
	Servers []Server `json:"servers"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken,omitempty"`
}

// AWSOIDCListSecurityGroupsRequest is a request to ListSecurityGroups using the AWS OIDC Integration.
type AWSOIDCListSecurityGroupsRequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// VPCID is the VPC to filter security groups by.
	VPCID string `json:"vpcId"`
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string `json:"nextToken"`
}

// AWSOIDCListSecurityGroupsResponse contains a list of VPC Security Groups and a next token if more pages are available.
type AWSOIDCListSecurityGroupsResponse struct {
	// SecurityGroups contains the page of SecurityGroups
	SecurityGroups []awsoidc.SecurityGroup `json:"securityGroups"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken,omitempty"`
}

// AWSOIDCRequiredVPCSRequest is a request to get required (missing) VPC's and its subnets.
type AWSOIDCRequiredVPCSRequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// AccountID is the AWS Account ID.
	AccountID string `json:"accountId"`
}

// AWSOIDCRequiredVPCSResponse returns a list of required VPC's and its subnets.
type AWSOIDCRequiredVPCSResponse struct {
	// VPCMapOfSubnets is a map of vpc ids and its subnets.
	// Will be empty if no vpc's are required.
	VPCMapOfSubnets map[string][]string `json:"vpcMapOfSubnets"`
}

// AWSOIDCListEC2ICERequest is a request to ListEC2ICEs using the AWS OIDC Integration.
type AWSOIDCListEC2ICERequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// VPCID is the VPC to filter EC2 Instance Connect Endpoints.
	// Deprecated: use VPCIDs instead.
	VPCID string `json:"vpcId"`
	// VPCIDs is a list of VPCs to filter EC2 Instance Connect Endpoints.
	VPCIDs []string `json:"vpcIds"`
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string `json:"nextToken"`
}

// AWSOIDCListEC2ICEResponse contains a list of AWS Instance Connect Endpoints and a next token if more pages are available.
type AWSOIDCListEC2ICEResponse struct {
	// EC2ICEs contains the page of Endpoints
	EC2ICEs []awsoidc.EC2InstanceConnectEndpoint `json:"ec2Ices"`

	// DashboardLink is the URL for AWS Web Console that lists all the Endpoints for the queries VPCs.
	DashboardLink string `json:"dashboardLink,omitempty"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken,omitempty"`
}

// AWSOIDCDeployEC2ICERequest is a request to create an AWS EC2 Instance Connect Endpoint.
type AWSOIDCDeployEC2ICERequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// SubnetID is the subnet id for the EC2 Instance Connect Endpoint.
	SubnetID string `json:"subnetId"`
	// SecurityGroupIDs is the list of SecurityGroups to apply to the Endpoint.
	// If not specified, the Endpoint will receive the default SG for the Subnet's VPC.
	SecurityGroupIDs []string `json:"securityGroupIds"`
}

// AWSOIDCDeployEC2ICEResponse is the response after creating an AWS EC2 Instance Connect Endpoint.
type AWSOIDCDeployEC2ICEResponse struct {
	// Name is the name of the endpoint that was created.
	Name string `json:"name"`
}
