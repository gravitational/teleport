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
import NodesService from 'teleport/services/nodes';

test('correct formatting of nodes fetch response', async () => {
  const nodesService = new NodesService();
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);
  const response = await nodesService.fetchNodes('does-not-matter');

  expect(response).toEqual({
    agents: [
      {
        kind: 'node',
        id: '00a53f99-993b-40bc-af51-5ba259af4e43',
        clusterId: 'im-a-cluster-name',
        hostname: 'im-a-nodename',
        labels: [{ name: 'env', value: 'dev' }],
        addr: '192.168.86.132:3022',
        tunnel: false,
        sshLogins: ['root'],
        subKind: 'teleport',
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
      subKind: 'teleport',
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
