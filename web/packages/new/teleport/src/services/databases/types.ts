import type { DbProtocol } from 'shared-new/services/databases/databases';

import type { ResourceLabel } from '../agents/types';
import type { AwsRdsDatabase, RdsEngine, Regions } from '../integrations/types';

export enum IamPolicyStatus {
  // Unspecified flag is most likely a result
  // from an older service that do not set this state
  Unspecified = 'IAM_POLICY_STATUS_UNSPECIFIED',
  Pending = 'IAM_POLICY_STATUS_PENDING',
  Failed = 'IAM_POLICY_STATUS_FAILED',
  Success = 'IAM_POLICY_STATUS_SUCCESS',
}

export interface Aws {
  rds: Pick<
    AwsRdsDatabase,
    'resourceId' | 'region' | 'subnets' | 'vpcId' | 'securityGroups'
  >;
  iamPolicyStatus: IamPolicyStatus;
}

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
}

export interface DatabasesResponse {
  databases: Database[];
  startKey?: string;
  totalCount?: number;
}

export type UpdateDatabaseRequest = Omit<
  Partial<CreateDatabaseRequest>,
  'protocol'
> & {
  caCert?: string;
};

export interface CreateDatabaseRequest {
  name: string;
  protocol: DbProtocol | RdsEngine;
  uri: string;
  labels?: ResourceLabel[];
  awsRds?: AwsRdsDatabase;
  awsRegion?: Regions;
  awsVpcId?: string;
  overwrite?: boolean;
}

export interface DatabaseIamPolicyResponse {
  type: string;
  aws: DatabaseIamPolicyAws;
}

export interface DatabaseIamPolicyAws {
  policy_document: string;
  placeholders: string;
}

export interface DatabaseService {
  // name is the name of the database service.
  name: string;
  // matcherLabels is a map of label keys with list of label values
  // that this service can discover databases with matching labels.
  matcherLabels: Record<string, string[]>;
}

export interface DatabaseServicesResponse {
  services: DatabaseService[];
  startKey?: string;
  totalCount?: number;
}
