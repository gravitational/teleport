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

import KubeService from './kube';

test('correct processed fetch response formatting', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockApiResponse);

  const kubeService = new KubeService();
  const response = await kubeService.fetchKubernetes('clusterId', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [
      {
        kind: 'kube_cluster',
        name: 'tele.logicoma.dev-prod',
        labels: [
          { name: 'kernal', value: '4.15.0-51-generic' },
          { name: 'env', value: 'prod' },
        ],
        users: [],
        groups: [],
      },
    ],
    startKey: mockApiResponse.startKey,
    totalCount: mockApiResponse.totalCount,
  });
});

test('handling of null fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const kubeService = new KubeService();
  const response = await kubeService.fetchKubernetes('clusterId', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [],
    startKey: undefined,
    totalCount: undefined,
  });
});

test('handling of null labels', async () => {
  jest
    .spyOn(api, 'get')
    .mockResolvedValue({ items: [{ name: 'test', labels: null }] });

  const kubeService = new KubeService();
  const response = await kubeService.fetchKubernetes('clusterId', {
    search: 'does-not-matter',
  });

  expect(response.agents).toEqual([
    { kind: 'kube_cluster', name: 'test', labels: [], users: [], groups: [] },
  ]);
});

const mockApiResponse = {
  items: [
    {
      name: 'tele.logicoma.dev-prod',
      labels: [
        { name: 'kernal', value: '4.15.0-51-generic' },
        { name: 'env', value: 'prod' },
      ],
      users: [],
      groups: [],
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
