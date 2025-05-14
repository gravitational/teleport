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

import { ResourceTargetHealth } from 'shared/components/UnifiedResources';
import { DbProtocol } from 'shared/services/databases';

import { ResourceLabel } from 'teleport/services/agents';

import { AwsRdsDatabase, RdsEngine, Regions } from '../integrations';

export enum IamPolicyStatus {
  // Unspecified flag is most likely a result
  // from an older service that do not set this state
  Unspecified = 'IAM_POLICY_STATUS_UNSPECIFIED',
  Pending = 'IAM_POLICY_STATUS_PENDING',
  Failed = 'IAM_POLICY_STATUS_FAILED',
  Success = 'IAM_POLICY_STATUS_SUCCESS',
}

export type Aws = {
  rds: Pick<
    AwsRdsDatabase,
    'resourceId' | 'region' | 'subnets' | 'vpcId' | 'securityGroups'
  >;
  iamPolicyStatus: IamPolicyStatus;
};

export interface Database {
  kind: 'db';
  name: string;
  description: string;
  type: string;
  protocol: DbProtocol;
  labels: ResourceLabel[];
  names?: string[];
  users?: string[];
  roles?: string[];
  hostname: string;
  aws?: Aws;
  requiresRequest?: boolean;
  supportsInteractive?: boolean;
  autoUsersEnabled?: boolean;
  /**
   * targetHealth describes the health status of network connectivity
   * reported from an agent (db_service) that is proxying this database.
   *
   * This field will be empty if the database was not extracted from
   * a db_server resource. The following endpoints will set this field
   * since these endpoints query for db_server under the hood and then
   * extract db from it:
   * - webapi/sites/:site/databases/:database (singular)
   * - webapi/sites/:site/resources (unified resources)
   */
  targetHealth?: ResourceTargetHealth;
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
  labels?: ResourceLabel[];
  awsRds?: AwsRdsDatabase;
  awsRegion?: Regions;
  awsVpcId?: string;
  overwrite?: boolean;
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

export type DatabaseServer = {
  hostname: string;
  hostId: string;
  targetHealth?: ResourceTargetHealth;
};
