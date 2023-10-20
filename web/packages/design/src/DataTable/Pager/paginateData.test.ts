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
