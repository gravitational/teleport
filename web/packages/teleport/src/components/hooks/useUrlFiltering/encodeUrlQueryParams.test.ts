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

import { encodeUrlQueryParams } from './encodeUrlQueryParams';

test.each([
  {
    title: 'No query params',
    args: ['/foo', '', null, null, false],
    expected: '/foo',
  },
  {
    title: 'Search string',
    args: ['/test', 'something', null, null, false],
    expected: '/test?search=something',
  },
  {
    title: 'Search string, encoded',
    args: ['/test', 'a$b$c', null, null, false],
    expected: '/test?search=a%24b%24c',
  },
  {
    title: 'Advanced search',
    args: ['/test', 'foo=="bar"', null, null, true],
    expected: '/test?query=foo%3D%3D%22bar%22',
  },
  {
    title: 'Search and sort',
    args: ['/test', 'foobar', { fieldName: 'name', dir: 'ASC' }, null, false],
    expected: '/test?search=foobar&sort=name%3Aasc',
  },
  {
    title: 'Sort only',
    args: ['/test', '', { fieldName: 'name', dir: 'ASC' }, null, false],
    expected: '/test?sort=name%3Aasc',
  },
  {
    title: 'Search, sort, and filter by kind',
    args: [
      '/test',
      'foo',
      { fieldName: 'name', dir: 'DESC' },
      ['db', 'node'],
      false,
    ],
    expected: '/test?search=foo&sort=name%3Adesc&kinds=db&kinds=node',
  },
])('$title', ({ args, expected }) => {
  expect(
    encodeUrlQueryParams(...(args as Parameters<typeof encodeUrlQueryParams>))
  ).toBe(expected);
});
