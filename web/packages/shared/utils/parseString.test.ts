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

import { parseQuotedWordsDelimitedByComma } from './parseString';

const tests: { name: string; str: string; expected: string[] }[] = [
  {
    name: 'empty input',
    str: '',
    expected: [],
  },
  {
    name: 'simple input',
    str: 'foo',
    expected: ['foo'],
  },
  {
    name: 'multi simple input',
    str: `"foo","bar","baz"`,
    expected: ['foo', 'bar', 'baz'],
  },
  {
    name: 'multi simple input with space',
    str: `"foo", "bar", "baz"`,
    expected: ['foo', 'bar', 'baz'],
  },
  {
    name: 'complex input',
    str: `"foo,bar","some phrase's",baz=qux's ,"some other  phrase"," another one  "`,
    expected: [
      'foo,bar',
      "some phrase's",
      "baz=qux's",
      'some other  phrase',
      'another one',
    ],
  },
  {
    name: 'unicode input',
    str: `"服务器环境=测试,操作系统类别", Linux , 机房=华北 `,
    expected: ['服务器环境=测试,操作系统类别', 'Linux', '机房=华北'],
  },
];

tests.forEach(test => {
  it(`${test.name}`, () => {
    const parsed = parseQuotedWordsDelimitedByComma(test.str);
    expect(parsed).toEqual(test.expected);
  });
});
