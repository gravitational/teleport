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

import api from 'teleport/services/api';

import clusterService from './clusters';

test('correct formatting of clusters fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);

  const response = await clusterService.fetchClusters();

  expect(response).toEqual([
    {
      clusterId: 'im-a-cluster-name',
      lastConnected: new Date('2022-02-02T14:03:00.355597-05:00'),
      connectedText: '2022-02-02 19:03:00',
      status: 'online',
      url: '/web/cluster/im-a-cluster-name/',
      authVersion: '8.0.0-alpha.1',
      nodeCount: 1,
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
    nodeCount: 1,
    publicURL: 'mockurl:3080',
    authVersion: '8.0.0-alpha.1',
    proxyVersion: '8.0.0-alpha.1',
  },
];
