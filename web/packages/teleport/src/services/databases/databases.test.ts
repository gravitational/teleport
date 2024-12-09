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

import api from 'teleport/services/api';

import DatabaseService from './databases';
import { Database, IamPolicyStatus } from './types';

test('correct formatting of database fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);

  const database = new DatabaseService();
  const response = await database.fetchDatabases('im-a-cluster', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [
      {
        kind: 'db',
        name: 'aurora',
        description: 'PostgreSQL 11.6: AWS Aurora',
        type: 'Amazon RDS PostgreSQL',
        protocol: 'postgres',
        names: [],
        users: [],
        roles: [],
        hostname: '',
        labels: [
          { name: 'cluster', value: 'root' },
          { name: 'env', value: 'aws' },
        ],
        aws: {
          rds: {
            resourceId: 'resource-id',
            region: 'us-west-1',
            vpcId: 'vpc-123',
            subnets: ['sn1', 'sn2'],
            securityGroups: [],
          },
          iamPolicyStatus: IamPolicyStatus.Success,
        },
        supportsInteractive: false,
      },
      {
        kind: 'db',
        name: 'self-hosted',
        type: 'Self-hosted PostgreSQL',
        protocol: 'postgres',
        names: [],
        users: [],
        roles: [],
        hostname: '',
        labels: [],
        aws: undefined,
        supportsInteractive: true,
      },
    ],
    startKey: mockResponse.startKey,
    totalCount: mockResponse.totalCount,
  });
});

test('null response from database fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const database = new DatabaseService();
  const response = await database.fetchDatabases('im-a-cluster', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [],
    startKey: undefined,
    totalCount: undefined,
  });
});

describe('correct formatting of all type and protocol combos', () => {
  test.each`
    type                     | protocol                 | combined
    ${'self-hosted'}         | ${'mysql'}               | ${'Self-hosted MySQL/MariaDB'}
    ${'rds'}                 | ${'mysql'}               | ${'Amazon RDS MySQL/MariaDB'}
    ${'self-hosted'}         | ${'postgres'}            | ${'Self-hosted PostgreSQL'}
    ${'rds'}                 | ${'postgres'}            | ${'Amazon RDS PostgreSQL'}
    ${'rdsproxy'}            | ${'sqlserver'}           | ${'Amazon RDS Proxy SQL Server'}
    ${'gcp'}                 | ${'postgres'}            | ${'Cloud SQL PostgreSQL'}
    ${'redshift'}            | ${'postgres'}            | ${'Amazon Redshift'}
    ${'redshift-serverless'} | ${'postgres'}            | ${'Amazon Redshift Serverless'}
    ${'dynamodb'}            | ${'dynamodb'}            | ${'Amazon DynamoDB'}
    ${'elasticache'}         | ${'redis'}               | ${'Amazon ElastiCache'}
    ${'memorydb'}            | ${'redis'}               | ${'Amazon MemoryDB'}
    ${'opensearch'}          | ${'opensearch'}          | ${'Amazon OpenSearch'}
    ${'keyspace'}            | ${'cassandra'}           | ${'Amazon Keyspaces'}
    ${'self-hosted'}         | ${'sqlserver'}           | ${'Self-hosted SQL Server'}
    ${'self-hosted'}         | ${'redis'}               | ${'Self-hosted Redis'}
    ${'self-hosted'}         | ${'mongodb'}             | ${'Self-hosted MongoDB'}
    ${'self-hosted'}         | ${'cassandra'}           | ${'Self-hosted Cassandra'}
    ${'self-hosted'}         | ${'cockroachdb'}         | ${'Self-hosted CockroachDB'}
    ${'self-hosted'}         | ${'oracle'}              | ${'Self-hosted Oracle'}
    ${'self-hosted'}         | ${'snowflake'}           | ${'Snowflake'}
    ${'self-hosted'}         | ${'elasticsearch'}       | ${'Self-hosted Elasticsearch'}
    ${'some other type'}     | ${'some other protocol'} | ${'some other type some other protocol'}
  `(
    'should combine type: $type and protocol: $protocol correctly',
    async ({ type, protocol, combined }) => {
      jest.spyOn(api, 'get').mockResolvedValue({ items: [{ type, protocol }] });

      const database = new DatabaseService();
      const response = await database.fetchDatabases('im-a-cluster', {
        search: 'does-not-matter',
      });

      const dbs = response.agents as Database[];
      expect(dbs[0].type).toBe(combined);
    }
  );
});

test('null labels field in database fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ items: [{ labels: null }] });

  const database = new DatabaseService();
  const response = await database.fetchDatabases('im-a-cluster', {
    search: 'does-not-matter',
  });

  expect(response.agents[0].labels).toEqual([]);
});

test('database services fetch response', async () => {
  const database = new DatabaseService();

  jest.spyOn(api, 'get').mockResolvedValue(mockServiceResponse);
  const response = await database.fetchDatabaseServices('im-a-cluster');
  expect(response.services).toEqual([
    {
      name: 'db-service-1',
      matcherLabels: {
        env: ['prod', 'env'],
        os: ['mac', 'ios', 'linux', 'windows'],
        tag: ['tag'],
        fruit: ['apple'],
      },
    },
    {
      name: 'db-service-2',
      matcherLabels: {},
    },
  ]);
});

test('null array fields in database services fetch response', async () => {
  const database = new DatabaseService();

  jest.spyOn(api, 'get').mockResolvedValue({});
  let response = await database.fetchDatabaseServices('im-a-cluster');
  expect(response.services).toEqual([]);

  jest.spyOn(api, 'get').mockResolvedValue({ items: [{ name: '' }] });
  response = await database.fetchDatabaseServices('im-a-cluster');
  expect(response.services).toEqual([{ name: '', matcherLabels: {} }]);
});

const mockResponse = {
  items: [
    // aws rds
    {
      name: 'aurora',
      desc: 'PostgreSQL 11.6: AWS Aurora',
      protocol: 'postgres',
      type: 'rds',
      uri: 'postgres-aurora-instance-1.c1xpjrob56xs.us-west-1.rds.amazonaws.com:5432',
      hostname: '',
      labels: [
        { name: 'cluster', value: 'root' },
        { name: 'env', value: 'aws' },
      ],
      aws: {
        rds: {
          resource_id: 'resource-id',
          region: 'us-west-1',
          vpc_id: 'vpc-123',
          subnets: ['sn1', 'sn2'],
          security_groups: [],
        },
        iam_policy_status: 'IAM_POLICY_STATUS_SUCCESS',
      },
    },
    // non-aws self-hosted
    {
      name: 'self-hosted',
      type: 'self-hosted',
      protocol: 'postgres',
      uri: 'localhost:5432',
      labels: [],
      hostname: '',
      supports_interactive: true,
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};

const mockServiceResponse = {
  items: [
    {
      name: 'db-service-1',
      resource_matchers: [
        {
          labels: { env: ['prod', 'env'], os: ['mac', 'ios'], fruit: 'apple' },
        },
        { labels: { os: ['linux', 'windows'], tag: ['tag'] } },
      ],
    },
    {
      name: 'db-service-2',
      resource_matchers: [],
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
