/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { formatSortType, parseSortType } from './sort';

describe('parseSortType', () => {
  it.each`
    input               | expected
    ${'name:asc'}       | ${{ fieldName: 'name', dir: 'ASC' }}
    ${'name:desc'}      | ${{ fieldName: 'name', dir: 'DESC' }}
    ${'NAME:asc'}       | ${{ fieldName: 'NAME', dir: 'ASC' }}
    ${'NAME:ASC'}       | ${{ fieldName: 'NAME', dir: 'ASC' }}
    ${''}               | ${null}
    ${'name'}           | ${{ fieldName: 'name', dir: 'ASC' }}
    ${':asc'}           | ${null}
    ${':desc'}          | ${null}
    ${'name:asc:blah'}  | ${{ fieldName: 'name', dir: 'ASC' }}
    ${'name:desc:blah'} | ${{ fieldName: 'name', dir: 'DESC' }}
    ${'name:blah'}      | ${{ fieldName: 'name', dir: 'ASC' }}
  `('should parse "$input"', ({ input, expected }) => {
    expect(parseSortType(input)).toEqual(expected);
  });
});

describe('formatSortType', () => {
  it.each`
    input                                 | expected
    ${{ fieldName: 'name', dir: 'ASC' }}  | ${'name:asc'}
    ${{ fieldName: 'name', dir: 'DESC' }} | ${'name:desc'}
    ${{ fieldName: '', dir: 'ASC' }}      | ${':asc'}
    ${{ fieldName: 'UPPER', dir: 'ASC' }} | ${'UPPER:asc'}
  `('should format $input', ({ input, expected }) => {
    expect(formatSortType(input)).toEqual(expected);
  });
});
