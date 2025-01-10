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

import {
  encodeUrlQueryParams,
  EncodeUrlQueryParamsProps,
} from './encodeUrlQueryParams';

const testCases: {
  title: string;
  args: EncodeUrlQueryParamsProps;
  expected: string;
}[] = [
  {
    title: 'No query params',
    args: { pathname: '/foo' },
    expected: '/foo?pinnedOnly=false',
  },
  {
    title: 'Search string',
    args: { pathname: '/test', searchString: 'something' },
    expected: '/test?search=something&pinnedOnly=false',
  },
  {
    title: 'Search string, encoded',
    args: { pathname: '/test', searchString: 'a$b$c' },
    expected: '/test?search=a%24b%24c&pinnedOnly=false',
  },
  {
    title: 'Advanced search',
    args: {
      pathname: '/test',
      searchString: 'foo=="bar"',
      isAdvancedSearch: true,
    },
    expected: '/test?query=foo%3D%3D%22bar%22&pinnedOnly=false',
  },
  {
    title: 'Search and sort',
    args: {
      pathname: '/test',
      searchString: 'foobar',
      sort: { fieldName: 'name', dir: 'ASC' },
    },
    expected: '/test?search=foobar&sort=name%3Aasc&pinnedOnly=false',
  },
  {
    title: 'Sort only',
    args: {
      pathname: '/test',
      sort: { fieldName: 'name', dir: 'ASC' },
    },
    expected: '/test?sort=name%3Aasc&pinnedOnly=false',
  },
  {
    title: 'Search, sort, and filter by kind',
    args: {
      pathname: '/test',
      searchString: 'foo',
      sort: { fieldName: 'name', dir: 'DESC' },
      kinds: ['db', 'node'],
    },
    expected:
      '/test?search=foo&sort=name%3Adesc&pinnedOnly=false&kinds=db&kinds=node',
  },
];

test.each(testCases)('$title', ({ args, expected }) => {
  expect(encodeUrlQueryParams(args)).toBe(expected);
});
