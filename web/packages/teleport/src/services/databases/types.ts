/*
Copyright 2021-2022 Gravitational, Inc.

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

import { DbProtocol } from 'shared/services/databases';

import { AgentLabel } from 'teleport/services/agents';

import { AwsRdsDatabase, RdsEngine } from '../integrations';

export type Aws = {
  rds?: Pick<AwsRdsDatabase, 'resourceId' | 'region' | 'subnets'>;
};

export interface Database {
  name: string;
  description: string;
  type: string;
  protocol: DbProtocol;
  labels: AgentLabel[];
  names?: string[];
  users?: string[];
  hostname: string;
  aws?: Aws;
}

export type DatabasesResponse = {
  databases: Database[];
  startKey?: string;
  totalCount?: number;
};

export type UpdateDatabaseRequest = Omit<
  Partial<CreateDatabaseRequest>,
  'protocol'
> & {
  caCert?: string;
};

export type CreateDatabaseRequest = {
  name: string;
  protocol: DbProtocol | RdsEngine;
  uri: string;
  labels?: AgentLabel[];
  awsRds?: AwsRdsDatabase;
};

export type DatabaseIamPolicyResponse = {
  type: string;
  aws: DatabaseIamPolicyAws;
};

export type DatabaseIamPolicyAws = {
  policy_document: string;
  placeholders: string;
};

export type DatabaseService = {
  // name is the name of the database service.
  name: string;
  // matcherLabels is a map of label keys with list of label values
  // that this service can discover databases with matching labels.
  matcherLabels: Record<string, string[]>;
};

export type DatabaseServicesResponse = {
  services: DatabaseService[];
  startKey?: string;
  totalCount?: number;
};
