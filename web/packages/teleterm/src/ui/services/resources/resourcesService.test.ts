/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { AmbiguousHostnameError, ResourcesService } from './resourcesService';

import type * as tsh from 'teleterm/services/tshd/types';

const server: tsh.Server = {
  uri: '/clusters/bar/servers/foo',
  tunnel: false,
  name: 'foo',
  hostname: 'foo',
  addr: 'localhost',
  labelsList: [],
};

const getServerByHostnameTests: Array<
  {
    name: string;
    getServersMockedValue: Awaited<ReturnType<tsh.TshClient['getServers']>>;
  } & (
    | { expectedServer: tsh.Server; expectedErr?: never }
    | { expectedErr: any; expectedServer?: never }
  )
> = [
  {
    name: 'returns a server when the hostname matches a single server',
    getServersMockedValue: {
      agentsList: [server],
      totalCount: 1,
      startKey: 'foo',
    },
    expectedServer: server,
  },
  {
    name: 'throws an error when the hostname matches multiple servers',
    getServersMockedValue: {
      agentsList: [server, server],
      totalCount: 2,
      startKey: 'foo',
    },
    expectedErr: AmbiguousHostnameError,
  },
  {
    name: 'returns nothing if the hostname does not match any servers',
    getServersMockedValue: {
      agentsList: [],
      totalCount: 0,
      startKey: 'foo',
    },
    expectedServer: undefined,
  },
];
test.each(getServerByHostnameTests)(
  'getServerByHostname $name',
  async ({ getServersMockedValue, expectedServer, expectedErr }) => {
    const tshClient: Partial<tsh.TshClient> = {
      getServers: jest.fn().mockResolvedValueOnce(getServersMockedValue),
    };
    const service = new ResourcesService(tshClient as tsh.TshClient);

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
    });
  }
);
