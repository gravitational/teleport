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
