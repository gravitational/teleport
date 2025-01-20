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
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/ui"
)

// IntegrationAWSOIDCSpec contain the specific fields for the `aws-oidc` subkind integration.
type IntegrationAWSOIDCSpec struct {
	// RoleARN is the role associated with the integration when SubKind is `aws-oidc`
	RoleARN string `json:"roleArn,omitempty"`

	// IssuerS3Bucket is the Issuer configured in AWS using an S3 Bucket.
	IssuerS3Bucket string `json:"issuerS3Bucket,omitempty"`
	// IssuerS3Prefix is the prefix for the bucket above.
	IssuerS3Prefix string `json:"issuerS3Prefix,omitempty"`

	// Audience is used to record a name of a plugin or a discover service in Teleport
	// that depends on this integration.
	// Audience value can be empty or configured with supported preset audience type.
	// Preset audience may impose specific behavior on the integration CRUD API,
	// such as preventing integration from update or deletion. Empty audience value
	// should be treated as a default and backward-compatible behavior of the integration.
	Audience string `json:"audience,omitempty"`
}

// IntegrationGitHub contains the specific fields for the `github` subkind integration.
type IntegrationGitHub struct {
	Organization string `json:"organization"`
}

// CheckAndSetDefaults for the aws oidc integration spec.
func (r *IntegrationAWSOIDCSpec) CheckAndSetDefaults() error {
	if r.RoleARN == "" {
		return trace.BadParameter("missing awsoidc.roleArn field")
	}

	// Either both empty or both are filled.
	if (r.IssuerS3Bucket == "") != (r.IssuerS3Prefix == "") {
		return trace.BadParameter("missing awsoidc s3 fields")
	}

	return nil
}

// IntegrationWithSummary describes Integration fields and the fields required to return the summary.
type IntegrationWithSummary struct {
	*Integration
	// AWSEC2 contains the summary for the AWS EC2 resources for this integration.
	AWSEC2 ResourceTypeSummary `json:"awsec2,omitempty"`
	// AWSRDS contains the summary for the AWS RDS resources and agents for this integration.
	AWSRDS ResourceTypeSummary `json:"awsrds,omitempty"`
	// AWSEKS contains the summary for the AWS EKS resources for this integration.
	AWSEKS ResourceTypeSummary `json:"awseks,omitempty"`
}

// ResourceTypeSummary contains the summary of the enrollment rules and found resources by the integration.
type ResourceTypeSummary struct {
	// RulesCount is the number of enrollment rules that are using this integration.
	// A rule is a matcher in a DiscoveryConfig that is being processed by a DiscoveryService.
	// If the DiscoveryService is not reporting any Status, it means it is not being processed and it doesn't count for the number of rules.
	// Example 1: a DiscoveryConfig with a matcher whose Type is "EC2" for two regions count as two EC2 rules.
	// Example 2: a DiscoveryConfig with a matcher whose Types is "EC2,RDS" for one regions count as one EC2 rule.
	// Example 3: a DiscoveryConfig with a matcher whose Types is "EC2,RDS", but has no DiscoveryService using it, it counts as 0 rules.
	RulesCount int `json:"rulesCount,omitempty"`
	// ResourcesFound contains the count of resources found by this integration.
	ResourcesFound int `json:"resourcesFound,omitempty"`
	// ResourcesEnrollmentFailed contains the count of resources that failed to enroll into the cluster.
	ResourcesEnrollmentFailed int `json:"resourcesEnrollmentFailed,omitempty"`
	// ResourcesEnrollmentSuccess contains the count of resources that succeeded to enroll into the cluster.
	ResourcesEnrollmentSuccess int `json:"resourcesEnrollmentSuccess,omitempty"`
	// DiscoverLastSync contains the time when this integration tried to auto-enroll resources.
	DiscoverLastSync *time.Time `json:"discoverLastSync,omitempty"`
	// ECSDatabaseServiceCount is the total number of DatabaseServices that were deployed into Amazon ECS.
	// Only applicable for AWS RDS resource summary.
	ECSDatabaseServiceCount int `json:"ecsDatabaseServiceCount,omitempty"`
}

// IntegrationDiscoveryRule describes a discovery rule associated with an integration.
type IntegrationDiscoveryRule struct {
	// ResourceType indicates the type of resource that this rule targets.
	// This is the same value that is set in DiscoveryConfig.AWS.<Matcher>.Types
	// Example: ec2, rds, eks
	ResourceType string `json:"resourceType,omitempty"`
	// Region where this rule applies to.
	Region string `json:"region,omitempty"`
	// LabelMatcher is the set of labels that are used to filter the resources before trying to auto-enroll them.
	LabelMatcher []ui.Label `json:"labelMatcher,omitempty"`
	// DiscoveryConfig is the name of the DiscoveryConfig that created this rule.
	DiscoveryConfig string `json:"discoveryConfig,omitempty"`
	// LastSync contains the time when this rule was used.
	// If empty, it indicates that the rule is not being used.
	LastSync *time.Time `json:"lastSync,omitempty"`
}

// IntegrationDiscoveryRules contains the list of discovery rules for a given Integration.
type IntegrationDiscoveryRules struct {
	// Rules is the list of integration rules.
	Rules []IntegrationDiscoveryRule `json:"rules"`
	// NextKey is the position to resume listing rules.
	NextKey string `json:"nextKey,omitempty"`
}

// Integration describes Integration fields
type Integration struct {
	// Name is the Integration name.
	Name string `json:"name,omitempty"`
	// SubKind is the Integration SubKind.
	SubKind string `json:"subKind,omitempty"`
	// AWSOIDC contains the fields for `aws-oidc` subkind integration.
	AWSOIDC *IntegrationAWSOIDCSpec `json:"awsoidc,omitempty"`
	// GitHub contains the fields for `github` subkind integration.
	GitHub *IntegrationGitHub `json:"github,omitempty"`
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

	if r.AWSOIDC != nil {
		if err := r.AWSOIDC.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	switch r.SubKind {
	case types.IntegrationSubKindGitHub:
		if r.GitHub == nil {
			return trace.BadParameter("missing spec for GitHub integrations")
		}
		if err := types.ValidateGitHubOrganizationName(r.GitHub.Organization); err != nil {
			return trace.Wrap(err)
		}
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
	if r.AWSOIDC != nil {
		if err := r.AWSOIDC.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// IntegrationsListResponse contains a list of Integrations.
// In case of exceeding the pagination limit (either via query param `limit` or the default 1000)
// a `nextToken` is provided and should be used to obtain the next page (as a query param `startKey`)
type IntegrationsListResponse struct {
	// Items is a list of resources retrieved.
	Items []*Integration `json:"items"`
	// NextKey is the position to resume listing events.
	NextKey string `json:"nextKey"`
}

// MakeIntegrations creates a UI list of Integrations.
func MakeIntegrations(igs []types.Integration) ([]*Integration, error) {
	uiList := make([]*Integration, 0, len(igs))

	for _, ig := range igs {
		uiIg, err := MakeIntegration(ig)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		uiList = append(uiList, uiIg)
	}

	return uiList, nil
}

// MakeIntegration creates a UI Integration representation.
func MakeIntegration(ig types.Integration) (*Integration, error) {
	ret := &Integration{
		Name:    ig.GetName(),
		SubKind: ig.GetSubKind(),
	}

	switch ig.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		var s3Bucket string
		var s3Prefix string

		if s3Location := ig.GetAWSOIDCIntegrationSpec().IssuerS3URI; s3Location != "" {
			issuerS3BucketURL, err := url.Parse(s3Location)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			s3Bucket = issuerS3BucketURL.Host
			s3Prefix = strings.TrimLeft(issuerS3BucketURL.Path, "/")
		}

		ret.AWSOIDC = &IntegrationAWSOIDCSpec{
			RoleARN:        ig.GetAWSOIDCIntegrationSpec().RoleARN,
			IssuerS3Bucket: s3Bucket,
			IssuerS3Prefix: s3Prefix,
			Audience:       ig.GetAWSOIDCIntegrationSpec().Audience,
		}
	case types.IntegrationSubKindGitHub:
		spec := ig.GetGitHubIntegrationSpec()
		if spec == nil {
			return nil, trace.BadParameter("missing spec for GitHub integrations")
		}
		ret.GitHub = &IntegrationGitHub{
			Organization: spec.Organization,
		}
	}

	return ret, nil
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
	// VPCID filters databases to only include those deployed in the VPC.
	VPCID string `json:"vpcId"`
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

	// VPCID is the VPCID where the service is going to be deployed.
	VPCID string `json:"vpcId"`

	// AccountID is the AWS Account ID.
	// Optional. sts.GetCallerIdentity is used if the value is not provided.
	AccountID string `json:"accountId"`

	// SubnetIDs associated with the Service.
	// If deploying a Database Service, you should use the SubnetIDs returned by the List Database API call.
	SubnetIDs []string `json:"subnetIds"`

	// SecurityGroups to apply to the service's network configuration.
	// If empty, the default security group for the VPC is going to be used.
	SecurityGroups []string `json:"securityGroups"`

	// TaskRoleARN is the AWS Role's ARN used within the Task execution.
	// Ensure the AWS Client's Role has `iam:PassRole` for this Role's ARN.
	// This can be either the ARN or the short name of the AWS Role.
	TaskRoleARN string `json:"taskRoleArn"`

	// DeploymentMode is the deployment configuration for the service.
	// This indicates what set of services should be deployed.
	DeploymentMode string `json:"deploymentMode"`

	// DatabaseAgentMatcherLabels are the labels to be used when deploying a Database Service.
	// Those are the resource labels that the Service will monitor and proxy connections to.
	DatabaseAgentMatcherLabels []ui.Label `json:"databaseAgentMatcherLabels"`
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

	// AccountID is the AWS account to deploy service to.
	AccountID string `json:"accountId"`

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

// AWSOIDCDeployedDatabaseService represents a Teleport Database Service that is deployed in Amazon ECS.
type AWSOIDCDeployedDatabaseService struct {
	// Name is the ECS Service name.
	Name string `json:"name,omitempty"`
	// DashboardURL is the link to the ECS Service in Amazon Web Console.
	DashboardURL string `json:"dashboardUrl,omitempty"`
	// ValidTeleportConfig returns whether this ECS Service has a valid Teleport Configuration for a deployed Database Service.
	// ECS Services with non-valid configuration require the user to take action on them.
	// No MatchingLabels are returned with an invalid configuration.
	ValidTeleportConfig bool `json:"validTeleportConfig,omitempty"`
	// MatchingLabels are the labels that are used by the Teleport Database Service to know which databases it should proxy.
	MatchingLabels []ui.Label `json:"matchingLabels,omitempty"`
}

// AWSOIDCListDeployedDatabaseServiceResponse is a list of Teleport Database Services that are deployed as ECS Services.
type AWSOIDCListDeployedDatabaseServiceResponse struct {
	// Services are the ECS Services.
	Services []AWSOIDCDeployedDatabaseService `json:"services"`
}

// AWSOIDCEnrollEKSClustersRequest is a request to ListEKSClusters using the AWS OIDC Integration.
type AWSOIDCEnrollEKSClustersRequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// ClusterNames are names of the EKS clusters to enroll
	ClusterNames []string `json:"clusterNames"`
	// EnableAppDiscovery specifies if Teleport Kubernetes App discovery should be enabled inside enrolled clusters.
	EnableAppDiscovery bool `json:"enableAppDiscovery"`
	// ExtraLabels added to the enrolled clusters.
	ExtraLabels []ui.Label `json:"extraLabels"`
}

// EKSClusterEnrollmentResult contains result/error for a single cluster enrollment.
type EKSClusterEnrollmentResult struct {
	// ClusterName is the name of EKS cluster that was enrolled.
	ClusterName string `json:"clusterName"`
	// ResourceId is the label with resource ID from the join token for the enrolled cluster, UI can check
	// if when enrolled cluster appears in Teleport by using this ID.
	ResourceId string `json:"resourceId"`
	// Error is an error message, if enrollment was not successful.
	Error string `json:"error"`
}

// AWSOIDCEnrollEKSClustersResponse is a response to enrolling EKS cluster
type AWSOIDCEnrollEKSClustersResponse struct {
	// Results contains enrollment result per EKS cluster.
	Results []EKSClusterEnrollmentResult `json:"results"`
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

// AWSOIDCListSubnetsRequest is a request to ListSubnets using the AWS OIDC Integration.
type AWSOIDCListSubnetsRequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// VPCID is the VPC to filter subnets by.
	VPCID string `json:"vpcId"`
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string `json:"nextToken"`
}

// AWSOIDCListSubnetsResponse contains a list of VPC subnets and a next token if
// more pages are available.
type AWSOIDCListSubnetsResponse struct {
	// Subnets contains the page of subnets
	Subnets []awsoidc.Subnet `json:"subnets"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken,omitempty"`
}

// AWSOIDCRequiredVPCSRequest is a request to list VPCs.
type AWSOIDCListVPCsRequest struct {
	// Region is the AWS Region.
	Region string `json:"region"`
	// AccountID is the AWS Account ID.
	AccountID string `json:"accountId"`
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string `json:"nextToken"`
}

// DatabaseEnrollmentVPC is a wrapper around [awsoidc.VPC] that also includes
// a link to the ECS service for a deployed Teleport database service in that
// VPC, if one exists.
type DatabaseEnrollmentVPC struct {
	awsoidc.VPC
	// ECSServiceDashboardURL is a link to the ECS service deployed for this
	// VPC, if one exists. Can be empty.
	ECSServiceDashboardURL string `json:"ecsServiceDashboardURL"`
}

// AWSOIDCDatabaseVPCsResponse contains a list of VPCs, including a link to
// an existing db service deployment if one exists, and a next token if more
// pages are available.
type AWSOIDCDatabaseVPCsResponse struct {
	// VPCs contains a page of VPCs.
	VPCs []DatabaseEnrollmentVPC `json:"vpcs"`

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

// AWSOIDCPingResponse contains the result of the Ping request.
// This response contains meta information about the current state of the Integration.
type AWSOIDCPingResponse struct {
	// AccountID number of the account that owns or contains the calling entity.
	AccountID string `json:"accountId"`
	// ARN associated with the calling entity.
	ARN string `json:"arn"`
	// UserID is the unique identifier of the calling entity.
	UserID string `json:"userId"`
}

// AWSOIDCPingRequest contains ping request fields.
type AWSOIDCPingRequest struct {
	// RoleARN is optional, and used for cases such as
	// pinging to check validity before upserting an
	// AWS OIDC integration.
	RoleARN string `json:"roleArn,omitempty"`
}

// AWSOIDCDeployEC2ICERequest contains request fields for creating an app server.
type AWSOIDCCreateAWSAppAccessRequest struct {
	// Labels added to the app server resource that will be created.
	Labels map[string]string `json:"labels"`
}
