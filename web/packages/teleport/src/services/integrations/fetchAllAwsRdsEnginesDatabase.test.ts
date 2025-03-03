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

import api from 'teleport/services/api';

import { integrationService, makeAwsDatabase } from './integrations';

const testCases: {
  name: string;
  fetch: 'instance' | 'cluster' | 'both';
  err?: string;
  instancesNextKey?: string;
  clustersNextKey?: string;
}[] = [
  {
    name: 'fetch only rds instances',
    fetch: 'instance',
  },
  {
    name: 'fetch only clusters instances',
    fetch: 'cluster',
  },
  {
    name: 'fetch both clusters and instances',
    fetch: 'both',
    instancesNextKey: 'instance-key',
    clustersNextKey: 'cluster-key',
  },
];

test.each(testCases)('$name', async tc => {
  let instances;
  let clusters;

  if (tc.fetch === 'cluster') {
    clusters = [
      {
        protocol: 'sql',
        name: 'rds-cluster',
      },
    ];
  }
  if (tc.fetch === 'instance') {
    instances = [
      {
        protocol: 'postgres',
        name: 'rds-instance',
      },
    ];
  }
  jest
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      databases: instances || [],
      nextToken: tc.instancesNextKey,
    })
    .mockResolvedValueOnce({
      databases: clusters || [],
      nextToken: tc.clustersNextKey,
    });

  const resp = await integrationService.fetchAllAwsRdsEnginesDatabases(
    'some-name',
    {
      region: 'us-east-1',
    }
  );

  expect(resp).toStrictEqual({
    databases: [
      ...(clusters ? clusters.map(makeAwsDatabase) : []),
      ...(instances ? instances.map(makeAwsDatabase) : []),
    ],
    instancesNextToken: tc.instancesNextKey,
    clustersNextToken: tc.clustersNextKey,
  });
});

test('failed to fetch both clusters and instances should throw error', async () => {
  jest.spyOn(api, 'post').mockRejectedValue(new Error('some error'));

  await expect(
    integrationService.fetchAllAwsRdsEnginesDatabases('some-name', {
      region: 'us-east-1',
    })
  ).rejects.toThrow('some error');
});

test('fetching instances but failed fetch clusters', async () => {
  const instance = {
    protocol: 'postgres',
    name: 'rds-instance',
  };
  jest
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      databases: [instance],
    })
    .mockRejectedValue(new Error('some error'));

  const resp = await integrationService.fetchAllAwsRdsEnginesDatabases(
    'some-name',
    {
      region: 'us-east-1',
    }
  );

  expect(resp).toStrictEqual({
    databases: [makeAwsDatabase(instance)],
    oneOfError: 'Failed to fetch RDS clusters: Error: some error',
    instancesNextToken: undefined,
  });
});

test('fetching clusters but failed fetch instances', async () => {
  const cluster = {
    protocol: 'postgres',
    name: 'rds-cluster',
  };
  jest
    .spyOn(api, 'post')
    .mockRejectedValueOnce(new Error('some error'))
    .mockResolvedValue({
      databases: [cluster],
    });

  const resp = await integrationService.fetchAllAwsRdsEnginesDatabases(
    'some-name',
    {
      region: 'us-east-1',
    }
  );

  expect(resp).toStrictEqual({
    databases: [makeAwsDatabase(cluster)],
    oneOfError: 'Failed to fetch RDS instances: Error: some error',
    clustersNextToken: undefined,
  });
});
