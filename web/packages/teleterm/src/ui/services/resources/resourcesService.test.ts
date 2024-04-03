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

import {
  makeDatabase,
  makeKube,
  makeServer,
  makeApp,
} from 'teleterm/services/tshd/testHelpers';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';

import {
  AmbiguousHostnameError,
  ResourceSearchError,
  ResourcesService,
} from './resourcesService';

import type { TshdClient } from 'teleterm/services/tshd';
import type * as tsh from 'teleterm/services/tshd/types';

describe('getServerByHostname', () => {
  const server: tsh.Server = makeServer();
  const getServerByHostnameTests: Array<
    {
      name: string;
      getServersMockedValue: ReturnType<TshdClient['getServers']>;
    } & (
      | { expectedServer: tsh.Server; expectedErr?: never }
      | { expectedErr: any; expectedServer?: never }
    )
  > = [
    {
      name: 'returns a server when the hostname matches a single server',
      getServersMockedValue: new MockedUnaryCall({
        agents: [server],
        totalCount: 1,
        startKey: 'foo',
      }),
      expectedServer: server,
    },
    {
      name: 'throws an error when the hostname matches multiple servers',
      getServersMockedValue: new MockedUnaryCall({
        agents: [server, server],
        totalCount: 2,
        startKey: 'foo',
      }),
      expectedErr: AmbiguousHostnameError,
    },
    {
      name: 'returns nothing if the hostname does not match any servers',
      getServersMockedValue: new MockedUnaryCall({
        agents: [],
        totalCount: 0,
        startKey: 'foo',
      }),
      expectedServer: undefined,
    },
  ];
  test.each(getServerByHostnameTests)(
    '$name',
    async ({ getServersMockedValue, expectedServer, expectedErr }) => {
      const tshClient: Partial<TshdClient> = {
        getServers: jest.fn().mockResolvedValueOnce(getServersMockedValue),
      };
      const service = new ResourcesService(tshClient as TshdClient);

      const promise = service.getServerByHostname('/clusters/bar', 'foo');

      if (expectedErr) {
        // eslint-disable-next-line jest/no-conditional-expect
        await expect(promise).rejects.toThrow(expectedErr);
      } else {
        // eslint-disable-next-line jest/no-conditional-expect
        await expect(promise).resolves.toStrictEqual(expectedServer);
      }

      expect(tshClient.getServers).toHaveBeenCalledWith({
        clusterUri: '/clusters/bar',
        query: 'name == "foo"',
        limit: 2,
        sort: null,
        sortBy: '',
        startKey: '',
        search: '',
        searchAsRoles: '',
      });
    }
  );
});

describe('searchResources', () => {
  it('returns settled promises for each resource type', async () => {
    const server = makeServer();
    const db = makeDatabase();
    const kube = makeKube();
    const app = makeApp();

    const tshClient: Partial<TshdClient> = {
      getServers: jest.fn().mockResolvedValueOnce(
        new MockedUnaryCall({
          agents: [server],
          totalCount: 1,
          startKey: '',
        })
      ),
      getDatabases: jest.fn().mockResolvedValueOnce(
        new MockedUnaryCall({
          agents: [db],
          totalCount: 1,
          startKey: '',
        })
      ),
      getKubes: jest.fn().mockResolvedValueOnce(
        new MockedUnaryCall({
          agents: [kube],
          totalCount: 1,
          startKey: '',
        })
      ),
      getApps: jest.fn().mockResolvedValueOnce(
        new MockedUnaryCall({
          agents: [app],
          totalCount: 1,
          startKey: '',
        })
      ),
    };
    const service = new ResourcesService(tshClient as TshdClient);

    const searchResults = await service.searchResources({
      clusterUri: '/clusters/foo',
      search: '',
      filters: [],
      limit: 10,
    });
    expect(searchResults).toHaveLength(4);

    const [actualServers, actualApps, actualDatabases, actualKubes] =
      searchResults;
    expect(actualServers).toEqual({
      status: 'fulfilled',
      value: [{ kind: 'server', resource: server }],
    });
    expect(actualApps).toEqual({
      status: 'fulfilled',
      value: [{ kind: 'app', resource: app }],
    });
    expect(actualDatabases).toEqual({
      status: 'fulfilled',
      value: [{ kind: 'database', resource: db }],
    });
    expect(actualKubes).toEqual({
      status: 'fulfilled',
      value: [{ kind: 'kube', resource: kube }],
    });
  });

  it('returns a single item if a filter is supplied', async () => {
    const server = makeServer();
    const tshClient: Partial<TshdClient> = {
      getServers: jest.fn().mockResolvedValueOnce(
        new MockedUnaryCall({
          agents: [server],
          totalCount: 1,
          startKey: '',
        })
      ),
    };
    const service = new ResourcesService(tshClient as TshdClient);

    const searchResults = await service.searchResources({
      clusterUri: '/clusters/foo',
      search: '',
      filters: ['node'],
      limit: 10,
    });
    expect(searchResults).toHaveLength(1);

    const [actualServers] = searchResults;
    expect(actualServers).toEqual({
      status: 'fulfilled',
      value: [{ kind: 'server', resource: server }],
    });
  });

  it('returns a custom error pointing at resource kind and cluster when an underlying promise gets rejected', async () => {
    const expectedCause = new Error('oops');
    const tshClient: Partial<TshdClient> = {
      getServers: jest.fn().mockRejectedValueOnce(expectedCause),
      getDatabases: jest.fn().mockRejectedValueOnce(expectedCause),
      getKubes: jest.fn().mockRejectedValueOnce(expectedCause),
      getApps: jest.fn().mockRejectedValueOnce(expectedCause),
    };
    const service = new ResourcesService(tshClient as TshdClient);

    const searchResults = await service.searchResources({
      clusterUri: '/clusters/foo',
      search: '',
      filters: [],
      limit: 10,
    });
    expect(searchResults).toHaveLength(4);

    const [actualServers, actualApps, actualDatabases, actualKubes] =
      searchResults;
    expect(actualServers).toEqual({
      status: 'rejected',
      reason: new ResourceSearchError('/clusters/foo', 'server', expectedCause),
    });
    expect(actualDatabases).toEqual({
      status: 'rejected',
      reason: new ResourceSearchError(
        '/clusters/foo',
        'database',
        expectedCause
      ),
    });
    expect(actualKubes).toEqual({
      status: 'rejected',
      reason: new ResourceSearchError('/clusters/foo', 'kube', expectedCause),
    });
    expect(actualApps).toEqual({
      status: 'rejected',
      reason: new ResourceSearchError('/clusters/foo', 'app', expectedCause),
    });

    expect((actualServers as PromiseRejectedResult).reason).toBeInstanceOf(
      ResourceSearchError
    );
    expect((actualServers as PromiseRejectedResult).reason.cause).toEqual(
      expectedCause
    );
  });
});
