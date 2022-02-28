/*
Copyright 2021 Gravitational, Inc.

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

import DatabaseService from './databases';
import api from 'teleport/services/api';

test('correct formatting of database fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);

  const database = new DatabaseService();
  const response = await database.fetchDatabases('im-a-cluster');

  expect(response).toEqual({
    databases: [
      {
        name: 'aurora',
        desc: 'PostgreSQL 11.6: AWS Aurora',
        title: 'RDS PostgreSQL',
        protocol: 'postgres',
        tags: ['cluster: root', 'env: aws'],
      },
    ],
    startKey: mockResponse.startKey,
    totalCount: mockResponse.totalCount,
  });
});

test('null response from database fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const database = new DatabaseService();
  const response = await database.fetchDatabases('im-a-cluster');

  expect(response).toEqual({
    databases: [],
    startKey: undefined,
    totalCount: undefined,
  });
});

describe('correct formatting of all type and protocol combos', () => {
  test.each`
    type                 | protocol                 | combined
    ${'self-hosted'}     | ${'mysql'}               | ${'Self-hosted MySQL'}
    ${'rds'}             | ${'mysql'}               | ${'RDS MySQL'}
    ${'self-hosted'}     | ${'postgres'}            | ${'Self-hosted PostgreSQL'}
    ${'rds'}             | ${'postgres'}            | ${'RDS PostgreSQL'}
    ${'gcp'}             | ${'postgres'}            | ${'Cloud SQL PostgreSQL'}
    ${'redshift'}        | ${'postgres'}            | ${'Redshift'}
    ${'some other type'} | ${'some other protocol'} | ${'some other type some other protocol'}
  `(
    'should combine type: $type and protocol: $protocol correctly',
    async ({ type, protocol, combined }) => {
      jest.spyOn(api, 'get').mockResolvedValue({ items: [{ type, protocol }] });

      const database = new DatabaseService();
      const response = await database.fetchDatabases('im-a-cluster');

      expect(response.databases[0].title).toBe(combined);
    }
  );
});

test('null labels field in database fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ items: [{ labels: null }] });

  const database = new DatabaseService();
  const response = await database.fetchDatabases('im-a-cluster');

  expect(response.databases[0].tags).toEqual([]);
});

const mockResponse = {
  items: [
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
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
