/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Label } from 'teleport/types';

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
  S extends Record<string, any> = IntegrationSpecAwsOidc
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
}
export type IntegrationSpecAwsOidc = {
  roleArn: string;
};

export enum IntegrationStatusCode {
  Unknown = 0,
  Running = 1,
  OtherError = 2,
  Unauthorized = 3,
  SlackNotInChannel = 10,
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

export type Plugin = Integration<'plugin', PluginKind, PluginSpec>;
export type PluginSpec = Record<string, never>; // currently no 'spec' fields exposed to the frontend
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
  | 'jamf';

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
  'me-south-1': 'Middle East (Bahrain)',
  'me-central-1': 'Middle East (UAE)',
  'sa-east-1': 'South America (São Paulo)',
};

export type Regions = keyof typeof awsRegionMap;

// RdsEngine are the expected backend string values,
// used when requesting lists of rds databases of the
// specified engine.
export type RdsEngine =
  | 'aurora' // (for MySQL 5.6-compatible Aurora)
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
  // name is the the Database's name.
  name: string;
  // uri contains the endpoint with port for connecting to this Database.
  uri: string;
  // resourceId is the AWS Region-unique, immutable identifier for the DB.
  resourceId: string;
  // accountId is the AWS account id.
  accountId: string;
  // labels contains this Instance tags.
  labels: Label[];
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
  };
};
