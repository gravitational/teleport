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
import desktops from 'teleport/services/desktops';

test('correct formatting of desktops fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);
  const response = await desktops.fetchDesktops('does-not-matter', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [
      {
        os: 'windows',
        name: 'DC1-teleport-demo',
        addr: '10.0.0.10',
        labels: [{ name: 'env', value: 'test' }],
        logins: ['Administrator'],
      },
    ],
    startKey: mockResponse.startKey,
    totalCount: mockResponse.totalCount,
  });
});

test('null response from desktops fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const response = await desktops.fetchDesktops('does-not-matter', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [],
    startKey: undefined,
    totalCount: undefined,
  });
});

test('null labels field in desktops fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ items: [{ labels: null }] });
  const response = await desktops.fetchDesktops('does-not-matter', {
    search: 'does-not-matter',
  });

  expect(response.agents[0].labels).toEqual([]);
});

const mockResponse = {
  items: [
    {
      addr: '10.0.0.10',
      labels: [{ name: 'env', value: 'test' }],
      name: 'DC1-teleport-demo',
      os: 'windows',
      logins: ['Administrator'],
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
