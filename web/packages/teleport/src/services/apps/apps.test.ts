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
import apps from 'teleport/services/apps';

test('correct formatting of apps fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);
  const response = await apps.fetchApps('does-not-matter');

  expect(response).toEqual({
    apps: [
      {
        id: 'cluster-id-app-name-app-name.example.com',
        name: 'app-name',
        description: 'some description',
        uri: 'http://localhost:3001',
        publicAddr: 'app-name.example.com',
        tags: ['env: dev'],
        clusterId: 'cluster-id',
        fqdn: 'app-name.example.com',
        launchUrl:
          '/web/launch/app-name.example.com/cluster-id/app-name.example.com',
        awsRoles: [],
        awsConsole: false,
      },
    ],
    startKey: mockResponse.startKey,
    totalCount: mockResponse.totalCount,
  });
});

test('null response from apps fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const response = await apps.fetchApps('does-not-matter');

  expect(response).toEqual({
    apps: [],
    startKey: undefined,
    totalCount: undefined,
  });
});

test('null labels field in apps fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ items: [{ labels: null }] });
  const response = await apps.fetchApps('does-not-matter');

  expect(response.apps[0].tags).toEqual([]);
});

const mockResponse = {
  items: [
    {
      awsConsole: false,
      clusterId: 'cluster-id',
      description: 'some description',
      fqdn: 'app-name.example.com',
      labels: [{ name: 'env', value: 'dev' }],
      name: 'app-name',
      publicAddr: 'app-name.example.com',
      uri: 'http://localhost:3001',
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
