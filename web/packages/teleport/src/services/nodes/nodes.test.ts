/**
 * Copyright 2022 Gravitational, Inc.
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

import api from 'teleport/services/api';
import NodesService from 'teleport/services/nodes';

test('correct formatting of nodes fetch response', async () => {
  const nodesService = new NodesService();
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);
  const response = await nodesService.fetchNodes('does-not-matter');

  expect(response).toEqual({
    agents: [
      {
        id: '00a53f99-993b-40bc-af51-5ba259af4e43',
        clusterId: 'im-a-cluster-name',
        hostname: 'im-a-nodename',
        labels: [{ name: 'env', value: 'dev' }],
        addr: '192.168.86.132:3022',
        tunnel: false,
        sshLogins: ['root'],
      },
    ],
    startKey: mockResponse.startKey,
    totalCount: mockResponse.totalCount,
  });
});

test('null response from nodes fetch', async () => {
  const nodesService = new NodesService();
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const response = await nodesService.fetchNodes('does-not-matter');

  expect(response).toEqual({
    agents: [],
    startKey: undefined,
    totalCount: undefined,
  });
});

test('null fields in nodes fetch response', async () => {
  const nodesService = new NodesService();
  jest.spyOn(api, 'get').mockResolvedValue({
    items: [{ tags: null, sshLogins: null }],
  });
  const response = await nodesService.fetchNodes('does-not-matter');

  expect(response.agents[0].labels).toEqual([]);
  expect(response.agents[0].sshLogins).toEqual([]);
});

const mockResponse = {
  items: [
    {
      addr: '192.168.86.132:3022',
      hostname: 'im-a-nodename',
      id: '00a53f99-993b-40bc-af51-5ba259af4e43',
      siteId: 'im-a-cluster-name',
      tags: [{ name: 'env', value: 'dev' }],
      tunnel: false,
      sshLogins: ['root'],
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
