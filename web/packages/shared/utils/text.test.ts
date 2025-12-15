/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { capitalizeFirstLetter, listToSentence, pluralize } from './text';

test('pluralize', () => {
  expect(pluralize(0, 'apple')).toBe('apples');
  expect(pluralize(1, 'apple')).toBe('apple');
  expect(pluralize(2, 'apple')).toBe('apples');
  expect(pluralize(undefined, 'apple')).toBe('apples');
  expect(pluralize(null, 'apple')).toBe('apples');
});

test('capitalizeFirstLetter', () => {
  expect(capitalizeFirstLetter('hello')).toBe('Hello');
  expect(capitalizeFirstLetter('')).toBe('');
});

describe('listToSentence()', () => {
  const testCases = [
    {
      name: 'no words',
      list: [],
      expected: '',
    },
    {
      name: 'one word',
      list: ['a'],
      expected: 'a',
    },
    {
      name: 'two words',
      list: ['a', 'b'],
      expected: 'a and b',
    },
    {
      name: 'three words',
      list: ['a', 'b', 'c'],
      expected: 'a, b and c',
    },
    {
      name: 'lost of words',
      list: ['a', 'b', 'c', 'd', 'e', 'f', 'g'],
      expected: 'a, b, c, d, e, f and g',
    },
  ];

  test.each(testCases)('$name', ({ list, expected }) => {
    const originalList = [...list];
    expect(listToSentence(list)).toEqual(expected);
    expect(list).toEqual(originalList);
  });
});
