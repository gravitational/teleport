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

import api from 'teleport/services/api';

import DatabaseService from './databases';
import { Database } from './types';

test('correct formatting of database fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);

  const database = new DatabaseService();
  const response = await database.fetchDatabases('im-a-cluster', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [
      {
        name: 'aurora',
        description: 'PostgreSQL 11.6: AWS Aurora',
        type: 'Amazon RDS PostgreSQL',
        protocol: 'postgres',
        names: [],
        users: [],
        labels: [
          { name: 'cluster', value: 'root' },
          { name: 'env', value: 'aws' },
        ],
        aws: {
          rds: {
            resourceId: 'resource-id',
            region: 'us-west-1',
            subnets: ['sn1', 'sn2'],
          },
        },
      },
      {
        name: 'self-hosted',
        type: 'Self-hosted PostgreSQL',
        protocol: 'postgres',
        names: [],
        users: [],
        labels: [],
        aws: {
          rds: {
            subnets: [],
          },
        },
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
      labels: [
        { name: 'cluster', value: 'root' },
        { name: 'env', value: 'aws' },
      ],
      aws: {
        rds: {
          resource_id: 'resource-id',
          region: 'us-west-1',
          subnets: ['sn1', 'sn2'],
        },
      },
    },
    // non-aws self-hosted
    {
      name: 'self-hosted',
      type: 'self-hosted',
      protocol: 'postgres',
      uri: 'localhost:5432',
      labels: [],
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
