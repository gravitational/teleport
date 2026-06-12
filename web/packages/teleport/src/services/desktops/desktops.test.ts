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
import desktops from 'teleport/services/desktops';

test('correct formatting of desktops fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);
  const response = await desktops.fetchDesktops('does-not-matter', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [
      {
        kind: 'windows_desktop',
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
