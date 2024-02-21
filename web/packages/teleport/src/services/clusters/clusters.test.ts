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

import ClustersService from './clusters';

test('correct formatting of clusters fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);
  const clustersService = new ClustersService();

  const response = await clustersService.fetchClusters();

  expect(response).toEqual([
    {
      clusterId: 'im-a-cluster-name',
      lastConnected: new Date('2022-02-02T14:03:00.355597-05:00'),
      connectedText: '2022-02-02 19:03:00',
      status: 'online',
      url: '/web/cluster/im-a-cluster-name/',
      authVersion: '8.0.0-alpha.1',
      publicURL: 'mockurl:3080',
      proxyVersion: '8.0.0-alpha.1',
    },
  ]);
});

const mockResponse = [
  {
    name: 'im-a-cluster-name',
    lastConnected: '2022-02-02T14:03:00.355597-05:00',
    status: 'online',
    publicURL: 'mockurl:3080',
    authVersion: '8.0.0-alpha.1',
    proxyVersion: '8.0.0-alpha.1',
  },
];
