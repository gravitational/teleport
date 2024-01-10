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
      users: ['staticUser1', 'staticUser2'],
      names: ['staticName1', 'staticName2'],
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
      },
      statusCode: IntegrationStatusCode.Running,
    },
  };
}

export const ComponentWrapper: React.FC<PropsWithChildren> = ({ children }) => (
  <TeleportProvider
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
    resourceSpec={getDbResourceSpec(
      DatabaseEngine.Postgres,
      DatabaseLocation.Aws
    )}
  >
    {children}
  </TeleportProvider>
);
