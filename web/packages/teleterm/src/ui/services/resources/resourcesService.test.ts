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

import type { TshdClient } from 'teleterm/services/tshd';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeApp,
  makeDatabase,
  makeKube,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import type * as tsh from 'teleterm/services/tshd/types';

import {
  AmbiguousHostnameError,
  ResourceSearchError,
  ResourcesService,
} from './resourcesService';

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
  it('returns a promise with resources', async () => {
    const server = makeServer();
    const db = makeDatabase();
    const kube = makeKube();
    const app = makeApp();

    const tshClient: Partial<TshdClient> = {
      listUnifiedResources: jest.fn().mockResolvedValueOnce(
        new MockedUnaryCall({
          resources: [
            {
              resource: { oneofKind: 'server', server },
            },
            {
              resource: { oneofKind: 'app', app },
            },
            {
              resource: { oneofKind: 'database', database: db },
            },
            {
              resource: { oneofKind: 'kube', kube },
            },
          ],
          nextKey: '',
        })
      ),
    };
    const service = new ResourcesService(tshClient as TshdClient);

    const searchResults = await service.searchResources({
      clusterUri: '/clusters/foo',
      search: '',
      filters: [],
      limit: 10,
      includeRequestable: true,
    });
    expect(searchResults).toHaveLength(4);

    const [actualServer, actualApp, actualDatabase, actualKube] = searchResults;
    expect(actualServer).toEqual({ kind: 'server', resource: server });
    expect(actualApp).toEqual({
      kind: 'app',
      resource: {
        ...app,
        addrWithProtocol: 'tcp://local-app.example.com:3000',
      },
    });
    expect(actualDatabase).toEqual({ kind: 'database', resource: db });
    expect(actualKube).toEqual({ kind: 'kube', resource: kube });
  });

  it('returns a custom error pointing at cluster when a promise gets rejected', async () => {
    const expectedCause = new Error('oops');
    const tshClient: Partial<TshdClient> = {
      listUnifiedResources: jest.fn().mockRejectedValueOnce(expectedCause),
    };
    const service = new ResourcesService(tshClient as TshdClient);

    const searchResults = service.searchResources({
      clusterUri: '/clusters/foo',
      search: '',
      filters: [],
      limit: 10,
      includeRequestable: true,
    });
    await expect(searchResults).rejects.toThrow(
      new ResourceSearchError('/clusters/foo', expectedCause)
    );
    await expect(searchResults).rejects.toThrow(ResourceSearchError);
  });
});
