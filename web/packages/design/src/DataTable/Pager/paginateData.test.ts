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

import paginateData from './paginateData';

test('paginates data correctly given pageSize', () => {
  const pageSize = 4;
  const paginatedData = paginateData(data, pageSize);

  expect(paginatedData).toEqual([
    [
      {
        hostname: 'host-a',
        addr: '192.168.7.1',
      },
      {
        hostname: 'host-b',
        addr: '192.168.7.2',
      },
      {
        hostname: 'host-c',
        addr: '192.168.7.3',
      },
      {
        hostname: 'host-d',
        addr: '192.168.7.4',
      },
    ],
    [
      {
        hostname: 'host-4',
        addr: '192.168.7.4',
      },
      {
        hostname: 'host-e',
        addr: '192.168.7.1',
      },
      {
        hostname: 'host-f',
        addr: '192.168.7.2',
      },
      {
        hostname: 'host-g',
        addr: '192.168.7.3',
      },
    ],
    [
      {
        hostname: 'host-h',
        addr: '192.168.7.4',
      },
      {
        hostname: 'host-i',
        addr: '192.168.7.4',
      },
    ],
  ]);
});

test('empty data set should return array with an empty array inside it', () => {
  const paginatedData = paginateData([], 5);

  expect(paginatedData).toEqual([[]]);
});

const data = [
  {
    hostname: 'host-a',
    addr: '192.168.7.1',
  },
  {
    hostname: 'host-b',
    addr: '192.168.7.2',
  },
  {
    hostname: 'host-c',
    addr: '192.168.7.3',
  },
  {
    hostname: 'host-d',
    addr: '192.168.7.4',
  },
  {
    hostname: 'host-4',
    addr: '192.168.7.4',
  },
  {
    hostname: 'host-e',
    addr: '192.168.7.1',
  },
  {
    hostname: 'host-f',
    addr: '192.168.7.2',
  },
  {
    hostname: 'host-g',
    addr: '192.168.7.3',
  },
  {
    hostname: 'host-h',
    addr: '192.168.7.4',
  },
  {
    hostname: 'host-i',
    addr: '192.168.7.4',
  },
];
