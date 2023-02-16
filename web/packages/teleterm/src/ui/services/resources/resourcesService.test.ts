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

const expectedServer: tsh.Server = {
  uri: '/clusters/bar/servers/foo',
  tunnel: false,
  name: 'foo',
  hostname: 'foo',
  addr: 'localhost',
  labelsList: [],
};

test('getServerByHostname returns a server when the hostname matches a single server', async () => {
  const tshClient: Partial<tsh.TshClient> = {
    getServers: jest.fn().mockResolvedValueOnce({
      agentsList: [expectedServer],
      totalCount: 1,
      startKey: 'foo',
    } as Awaited<ReturnType<tsh.TshClient['getServers']>>),
  };
  const service = new ResourcesService(tshClient as tsh.TshClient);

  const actualServer = await service.getServerByHostname(
    '/clusters/bar',
    'foo'
  );
  expect(actualServer).toStrictEqual(expectedServer);
  expect(tshClient.getServers).toHaveBeenCalledWith({
    clusterUri: '/clusters/bar',
    query: 'name == "foo"',
    limit: 2,
  });
});

test('getServerByHostname returns an error when the hostname matches multiple servers', async () => {
  const tshClient: Partial<tsh.TshClient> = {
    getServers: jest.fn().mockResolvedValueOnce({
      agentsList: [expectedServer, expectedServer],
      totalCount: 2,
      startKey: 'foo',
    } as Awaited<ReturnType<tsh.TshClient['getServers']>>),
  };
  const service = new ResourcesService(tshClient as tsh.TshClient);

  await expect(
    service.getServerByHostname('/clusters/bar', 'foo')
  ).rejects.toThrow(AmbiguousHostnameError);

  expect(tshClient.getServers).toHaveBeenCalledWith({
    clusterUri: '/clusters/bar',
    query: 'name == "foo"',
    limit: 2,
  });
});

test('getServerByHostname returns nothing if the hostname does not match any servers', async () => {
  const tshClient: Partial<tsh.TshClient> = {
    getServers: jest.fn().mockResolvedValueOnce({
      agentsList: [],
      totalCount: 0,
      startKey: 'foo',
    } as Awaited<ReturnType<tsh.TshClient['getServers']>>),
  };
  const service = new ResourcesService(tshClient as tsh.TshClient);

  const actualServer = await service.getServerByHostname(
    '/clusters/bar',
    'foo'
  );

  expect(actualServer).toBeFalsy();
  expect(tshClient.getServers).toHaveBeenCalledWith({
    clusterUri: '/clusters/bar',
    query: 'name == "foo"',
    limit: 2,
  });
});
