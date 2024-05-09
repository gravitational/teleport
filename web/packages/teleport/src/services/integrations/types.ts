/**
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

import { Label } from 'teleport/types';

import { Node } from '../nodes';

/**
 * type Integration v. type Plugin:
 *
 * Before "integration" resource was made, a "plugin" resource existed.
 * They are essentially the same where plugin resource could've
 * been defined with the integration resource. But it's too late for
 * renames/changes. There are small differences between the two resource,
 * so they are separate types.
 *
 * "integration" resource is supported in both OS and Enterprise
 * while "plugin" resource is only supported in enterprise. Plugin
 * type exists in OS for easier typing when combining the resources
 * into one list.
 */
export type Integration<
  T extends string = 'integration',
  K extends string = IntegrationKind,
  S extends Record<string, any> = IntegrationSpecAwsOidc,
> = {
  resourceType: T;
  kind: K;
  spec: S;
  name: string;
  details?: string;
  statusCode: IntegrationStatusCode;
};
// IntegrationKind string values should be in sync
// with the backend value for defining the integration
// resource's subKind field.
export enum IntegrationKind {
  AwsOidc = 'aws-oidc',
  ExternalAuditStorage = 'external-audit-storage',
}
export type IntegrationSpecAwsOidc = {
  roleArn: string;
  issuerS3Prefix: string;
  issuerS3Bucket: string;
};

export enum IntegrationStatusCode {
  Unknown = 0,
  Running = 1,
  OtherError = 2,
  Unauthorized = 3,
  SlackNotInChannel = 10,
  Draft = 100,
}

export function getStatusCodeTitle(code: IntegrationStatusCode): string {
  switch (code) {
    case IntegrationStatusCode.Unknown:
      return 'Unknown';
    case IntegrationStatusCode.Running:
      return 'Running';
    case IntegrationStatusCode.Unauthorized:
      return 'Unauthorized';
    case IntegrationStatusCode.SlackNotInChannel:
      return 'Bot not invited to channel';
    case IntegrationStatusCode.Draft:
      return 'Draft';
    default:
      return 'Unknown error';
  }
}

export function getStatusCodeDescription(
  code: IntegrationStatusCode
): string | null {
  switch (code) {
    case IntegrationStatusCode.Unauthorized:
      return 'The integration was denied access. This could be a result of revoked authorization on the 3rd party provider. Try removing and re-connecting the integration.';

    case IntegrationStatusCode.SlackNotInChannel:
      return 'The Slack integration must be invited to the default channel in order to receive access request notifications.';
    default:
      return null;
  }
}

export type ExternalAuditStorage = {
  integrationName: string;
  policyName: string;
  region: string;
  sessionsRecordingsURI: string;
  athenaWorkgroup: string;
  glueDatabase: string;
  glueTable: string;
  auditEventsLongTermURI: string;
  athenaResultsURI: string;
};

export type ExternalAuditStorageIntegration = Integration<
  'external-audit-storage',
  IntegrationKind.ExternalAuditStorage,
  ExternalAuditStorage
>;

export type Plugin<T = any> = Integration<'plugin', PluginKind, T>;
export type PluginSpec = PluginOktaSpec | any; // currently only okta has a plugin spec
// PluginKind represents the type of the plugin
// and should be the same value as defined in the backend (check master branch for the latest):
// https://github.com/gravitational/teleport/blob/a410acef01e0023d41c18ca6b0a7b384d738bb32/api/types/plugin.go#L27
export type PluginKind =
  | 'slack'
  | 'openai'
  | 'pagerduty'
  | 'email'
  | 'jira'
  | 'discord'
  | 'mattermost'
  | 'msteams'
  | 'opsgenie'
  | 'okta'
  | 'servicenow'
  | 'jamf';

export type PluginOktaSpec = {
  // scimBearerToken is the plain text of the bearer token that Okta will use
  // to authenticate SCIM requests
  scimBearerToken: string;
  // oktaAppID is the Okta ID of the SAML App created during the Okta plugin
  // installation
  oktaAppId: string;
  // oktaAppName is the human readable name of the Okta SAML app created
  // during the Okta plugin installation
  oktaAppName: string;
  // teleportSSOConnector is the name of the Teleport SAML SSO connector
  // created by the plugin during installation
  teleportSsoConnector: string;
  // error contains a description of any failures during plugin installation
  // that were deemed not serious enough to fail the plugin installation, but
  // may effect the operation of advanced features like User Sync or SCIM.
  error: string;
};

export type IntegrationCreateRequest = {
  name: string;
  subKind: IntegrationKind;
  awsoidc?: IntegrationSpecAwsOidc;
};

export type IntegrationListResponse = {
  items: Integration[];
  nextKey?: string;
};

// awsRegionMap maps the AWS regions to it's region name
// as defined in (omitted gov cloud regions):
// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.RegionsAndAvailabilityZones.html
export const awsRegionMap = {
  'us-east-2': 'US East (Ohio)',
  'us-east-1': 'US East (N. Virginia)',
  'us-west-1': 'US West (N. California)',
  'us-west-2': 'US West (Oregon)',
  'af-south-1': 'Africa (Cape Town)',
  'ap-east-1': 'Asia Pacific (Hong Kong)',
  'ap-south-2': 'Asia Pacific (Hyderabad)',
  'ap-southeast-3': 'Asia Pacific (Jakarta)',
  'ap-southeast-4': 'Asia Pacific (Melbourne)',
  'ap-south-1': 'Asia Pacific (Mumbai)',
  'ap-northeast-3': 'Asia Pacific (Osaka)',
  'ap-northeast-2': 'Asia Pacific (Seoul)',
  'ap-southeast-1': 'Asia Pacific (Singapore)',
  'ap-southeast-2': 'Asia Pacific (Sydney)',
  'ap-northeast-1': 'Asia Pacific (Tokyo)',
  'ca-central-1': 'Canada (Central)',
  'eu-central-1': 'Europe (Frankfurt)',
  'eu-west-1': 'Europe (Ireland)',
  'eu-west-2': 'Europe (London)',
  'eu-south-1': 'Europe (Milan)',
  'eu-west-3': 'Europe (Paris)',
  'eu-south-2': 'Europe (Spain)',
  'eu-north-1': 'Europe (Stockholm)',
  'eu-central-2': 'Europe (Zurich)',
  'il-central-1': 'Israel (Tel Aviv)',
  'me-south-1': 'Middle East (Bahrain)',
  'me-central-1': 'Middle East (UAE)',
  'sa-east-1': 'South America (São Paulo)',
};

export type Regions = keyof typeof awsRegionMap;

// RdsEngine are the expected backend string values,
// used when requesting lists of rds databases of the
// specified engine.
export type RdsEngine =
  | 'aurora-mysql' // (for MySQL 5.7-compatible and MySQL 8.0-compatible Aurora)
  | 'aurora-postgresql'
  | 'mariadb'
  | 'mysql'
  | 'postgres';

// RdsEngineIdentifier are the name of engines
// used to determine the grouping of similar RdsEngines.
// eg: if `aurora-mysql` then the grouping of RdsEngines
// is 'aurora, aurora-mysql`, they are both mysql but
// refer to different versions. This type is used solely
// for frontend.
export type RdsEngineIdentifier =
  | 'mysql'
  | 'postgres'
  | 'aurora-mysql'
  | 'aurora-postgres';

export type AwsOidcListDatabasesRequest = {
  // engines is used as a filter to get a list of specified engines only.
  engines: RdsEngine[];
  region: Regions;
  // nextToken is the start key for the next page
  nextToken?: string;
  // rdsType describes the type of RDS dbs to request.
  // `cluster` is used for requesting aurora related
  // engines, and `instance` for rest of engines.
  rdsType: 'instance' | 'cluster';
};

export type AwsRdsDatabase = {
  // engine of the database. eg. aurora-mysql
  engine: RdsEngine;
  // name is the Database's name.
  name: string;
  // uri contains the endpoint with port for connecting to this Database.
  uri: string;
  // resourceId is the AWS Region-unique, immutable identifier for the DB.
  resourceId: string;
  // accountId is the AWS account id.
  accountId: string;
  // labels contains this Instance tags.
  labels: Label[];
  // subnets is a list of subnets for the RDS instance.
  subnets: string[];
  // vpcId is the AWS VPC ID for the DB.
  vpcId: string;
  // region is the AWS cloud region that this database is from.
  region: Regions;
  // status contains this Instance status.
  // There is a lot of status states available so only a select few were
  // hard defined to use to determine the status color.
  // https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html
  status: 'available' | 'failed' | 'deleting';
};

export type ListAwsRdsDatabaseResponse = {
  databases: AwsRdsDatabase[];
  // nextToken is the start key for the next page.
  // Empty value means last page.
  nextToken?: string;
};

export type IntegrationUpdateRequest = {
  awsoidc: {
    roleArn: string;
    issuerS3Bucket: string;
    issuerS3Prefix: string;
  };
};

export type AwsOidcDeployServiceRequest = {
  deploymentMode: 'database-service';
  region: Regions;
  subnetIds: string[];
  taskRoleArn: string;
  databaseAgentMatcherLabels: Label[];
  securityGroups?: string[];
};

// DeployDatabaseServiceDeployment identifies the required fields to deploy a DatabaseService.
type DeployDatabaseServiceDeployment = {
  // VPCID is the VPCID where the service is going to be deployed.
  vpcId: string;
  // SubnetIDs are the subnets for the network configuration.
  // They must belong to the VPCID above.
  subnetIds: string[];
  // SecurityGroups are the SecurityGroup IDs to associate with this particular deployment.
  // If empty, the default security group for the VPC is going to be used.
  // TODO(lisa): out of scope.
  securityGroups?: string[];
};

// AwsOidcDeployDatabaseServicesRequest contains the required fields to perform a DeployService request.
// Each deployed DatabaseService will be proxying the resources that match the following labels:
// -region: <Region>
// -account-id: <AccountID>s
// -vpc-id: <Deployments[].VPCID>
export type AwsOidcDeployDatabaseServicesRequest = {
  // Region is the AWS Region for the Service.
  region: string;
  // TaskRoleARN is the AWS Role's ARN used within the Task execution.
  // Ensure the AWS Client's Role has `iam:PassRole` for this Role's ARN.
  // This can be either the ARN or the short name of the AWS Role.
  taskRoleArn: string;
  // Deployments is a list of Services to be deployed.
  // If the target deployment already exists, the deployment is skipped.
  deployments: DeployDatabaseServiceDeployment[];
};

export type AwsEksCluster = {
  name: string;
  region: Regions;
  accountId: string;
  status:
    | 'active'
    | 'pending'
    | 'creating'
    | 'failed'
    | 'updating'
    | 'deleting';
  /**
   * labels contains this cluster's tags.
   */
  labels: Label[];
  /**
   * joinLabels contains labels that should be injected into teleport kube agent, if EKS cluster is being enrolled.
   */
  joinLabels: Label[];
};

export type EnrollEksClustersRequest = {
  region: string;
  enableAppDiscovery: boolean;
  clusterNames: string[];
};

export type EnrollEksClustersResponse = {
  results: {
    clusterName: string;
    resourceId: string;
    error: { message: string };
  }[];
};

export type ListEksClustersRequest = {
  region: Regions;
  nextToken?: string;
};

export type ListEksClustersResponse = {
  /**
   * clusters is the list of EKS clusters.
   */
  clusters: AwsEksCluster[];
  nextToken?: string;
};

export type ListEc2InstancesRequest = {
  region: Regions;
  nextToken?: string;
};

export type ListEc2InstancesResponse = {
  // instances is the list of EC2 Instances.
  instances: Node[];
  nextToken?: string;
};

export type ListEc2InstanceConnectEndpointsRequest = {
  region: Regions;
  // VPCIDs is a list of VPCs to filter EC2 Instance Connect Endpoints.
  vpcIds: string[];
  nextToken?: string;
};

export type ListEc2InstanceConnectEndpointsResponse = {
  // endpoints is the list of EC2 Instance Connect Endpoints.
  endpoints: Ec2InstanceConnectEndpoint[];
  nextToken?: string;
  // DashboardLink is the URL for AWS Web Console that
  // lists all the Endpoints for the queries VPCs.
  dashboardLink: string;
};

export type Ec2InstanceConnectEndpoint = {
  name: string;
  // state is the current state of the EC2 Instance Connect Endpoint.
  state: Ec2InstanceConnectEndpointState;
  // stateMessage is an optional message describing the state of the EICE, such as an error message.
  stateMessage?: string;
  // dashboardLink is a URL to AWS Console where the user can see the EC2 Instance Connect Endpoint.
  dashboardLink: string;
  // subnetID is the subnet used by the Endpoint. Please note that the Endpoint should be able to reach any subnet within the VPC.
  subnetId: string;
  // VPCID is the VPC ID where the Endpoint is created.
  vpcId: string;
};

export type Ec2InstanceConnectEndpointState =
  | 'create-in-progress'
  | 'create-complete'
  | 'create-failed'
  | 'delete-in-progress'
  | 'delete-complete'
  | 'delete-failed';

export type AwsOidcDeployEc2InstanceConnectEndpointRequest = {
  // SubnetID is the subnet id for the EC2 Instance Connect Endpoint.
  subnetId: string;
  // SecurityGroupIDs is the list of SecurityGroups to apply to the Endpoint.
  // If not specified, the Endpoint will receive the default SG for the Subnet's VPC.
  securityGroupIds?: string[];
};

export type DeployEc2InstanceConnectEndpointRequest = {
  region: Regions;
  // Endpoints is a list of endpoinst to create.
  endpoints: AwsOidcDeployEc2InstanceConnectEndpointRequest[];
};

export type AwsEc2InstanceConnectEndpoint = {
  // Name is the EC2 Instance Connect Endpoint name.
  name: string;
  // SubnetID is the subnet where this endpoint was created.
  subnetId: string;
};

export type DeployEc2InstanceConnectEndpointResponse = {
  // Endpoints is a list of created endpoints
  endpoints: AwsEc2InstanceConnectEndpoint[];
};

export type ListAwsSecurityGroupsRequest = {
  // VPCID is the VPC to filter Security Groups.
  vpcId: string;
  region: Regions;
  nextToken?: string;
};

export type ListAwsSecurityGroupsResponse = {
  securityGroups: SecurityGroup[];
  nextToken?: string;
};

export type SecurityGroup = {
  // Name is the Security Group name.
  // This is just a friendly name and should not be used for further API calls
  name: string;
  // ID is the security group ID.
  // This is the value that should be used when doing further API calls.
  id: string;
  description: string;
  // InboundRules describe the Security Group Inbound Rules.
  // The CIDR of each rule represents the source IP that the rule applies to.
  inboundRules: SecurityGroupRule[];
  // OutboundRules describe the Security Group Outbound Rules.
  // The CIDR of each rule represents the destination IP that the rule applies to.
  outboundRules: SecurityGroupRule[];
};

export type SecurityGroupRule = {
  // IPProtocol is the protocol used to describe the rule.
  ipProtocol: string;
  // FromPort is the inclusive start of the Port range for the Rule.
  fromPort: string;
  // ToPort is the inclusive end of the Port range for the Rule.
  toPort: string;
  // CIDRs contains a list of IP ranges that this rule applies to and a description for the value.
  cidrs: Cidr[];
};

export type Cidr = {
  // CIDR is the IP range using CIDR notation.
  cidr: string;
  // Description contains a small text describing the CIDR.
  description: string;
};

// IntegrationUrlLocationState define fields to preserve state between
// react routes (eg. in External Audit Storage flow, it is required of user
// to create a AWS OIDC integration which requires changing route
// and then coming back to resume the flow.)
export type IntegrationUrlLocationState = {
  kind: IntegrationKind;
  redirectText: string;
};
