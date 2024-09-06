/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { PropsWithChildren } from 'react';

import {
  DatabaseEngine,
  DatabaseLocation,
  ResourceSpec,
} from 'teleport/Discover/SelectResource';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { DbMeta } from 'teleport/Discover/useDiscover';
import { IamPolicyStatus } from 'teleport/services/databases';

import { DATABASES } from '../SelectResource/databases';
import { ResourceKind } from '../Shared';

import { TeleportProvider } from './fixtures';

export function getDbResourceSpec(
  engine: DatabaseEngine,
  location?: DatabaseLocation
): ResourceSpec {
  return {
    ...DATABASES[0],
    dbMeta: {
      engine,
      location,
    },
  };
}

export function getDbMeta(): DbMeta {
  return {
    resourceName: 'db-name',
    awsRegion: 'us-east-1',
    agentMatcherLabels: [],
    db: {
      aws: {
        iamPolicyStatus: IamPolicyStatus.Unspecified,
        rds: {
          region: 'us-east-1',
          vpcId: 'test-vpc',
          resourceId: 'some-rds-resource-id',
          subnets: [],
        },
      },
      kind: 'db',
      name: 'some-db-name',
      description: 'some-description',
      type: 'rds',
      protocol: 'postgres',
      labels: [],
      hostname: 'some-db-hostname',
      users: ['staticUser1', 'staticUser2', '*'],
      names: ['staticName1', 'staticName2', '*'],
    },
    selectedAwsRdsDb: {
      region: 'us-east-1',
      engine: 'postgres',
      name: 'rds-1',
      uri: '',
      resourceId: 'some-rds-resource-id',
      accountId: '1234',
      labels: [],
      subnets: [],
      vpcId: '',
      status: 'available',
    },
    awsIntegration: {
      kind: IntegrationKind.AwsOidc,
      name: 'test-integration',
      resourceType: 'integration',
      spec: {
        roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
        issuerS3Bucket: '',
        issuerS3Prefix: '',
      },
      statusCode: IntegrationStatusCode.Running,
    },
  };
}

export const ComponentWrapper: React.FC<
  PropsWithChildren<{ resourceSpec?: ResourceSpec; dbMeta?: DbMeta }>
> = ({ children, resourceSpec, dbMeta }) => (
  <TeleportProvider
    agentMeta={dbMeta || getDbMeta()}
    resourceKind={ResourceKind.Database}
    resourceSpec={
      resourceSpec ||
      getDbResourceSpec(DatabaseEngine.Postgres, DatabaseLocation.Aws)
    }
  >
    {children}
  </TeleportProvider>
);
