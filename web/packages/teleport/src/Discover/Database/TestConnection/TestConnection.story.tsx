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

import { Info } from 'design/Alert';

import {
  resourceSpecAwsRdsMySql,
  resourceSpecAwsRdsPostgres,
} from 'teleport/Discover/Fixtures/databases';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';
import { AgentMeta } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/Database/TestConnection',
};

const agentMeta: AgentMeta = {
  resourceName: 'db-name',
  agentMatcherLabels: [],
  db: {
    kind: 'db',
    name: 'postgres',
    description: 'PostgreSQL 11.6: AWS postgres ',
    type: 'RDS PostgreSQL',
    protocol: 'postgres',
    labels: [
      { name: 'cluster', value: 'root' },
      { name: 'env', value: 'aws' },
    ],
    hostname: 'postgres-hostname',
    names: ['name1', 'name2', '*'],
    users: ['user1', 'user2', '*'],
  },
  awsIntegration: {
    kind: IntegrationKind.AwsOidc,
    name: 'test-oidc',
    resourceType: 'integration',
    spec: {
      roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
      issuerS3Bucket: '',
      issuerS3Prefix: '',
    },
    statusCode: IntegrationStatusCode.Running,
  },
};

export const InitMySql = () => (
  <RequiredDiscoverProviders
    agentMeta={agentMeta}
    resourceSpec={resourceSpecAwsRdsMySql}
  >
    <Info>Devs: mysql allows database names to be empty</Info>
    <TestConnection />
  </RequiredDiscoverProviders>
);

export const InitPostgres = () => (
  <RequiredDiscoverProviders
    agentMeta={agentMeta}
    resourceSpec={resourceSpecAwsRdsPostgres}
  >
    <TestConnection />
  </RequiredDiscoverProviders>
);
