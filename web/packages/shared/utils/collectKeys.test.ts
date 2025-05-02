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

import { collectKeys } from './collectKeys';

describe('collectKeys', () => {
  it.each`
    value
    ${undefined}
    ${null}
    ${1}
    ${true}
    ${() => {}}
  `('supports non object values ($value)', ({ value }) => {
    const actual = collectKeys(value);
    expect(actual).toBeNull();
  });

  it('supports empty object values', () => {
    const actual = collectKeys({});
    expect(actual).toEqual([]);
  });

  it('supports empty array values', () => {
    const actual = collectKeys([]);
    expect(actual).toEqual([]);
  });

  it('supports simple object values', () => {
    const actual = collectKeys({
      number: 1,
      boolean: true,
      string: 'string',
      function: () => {},
      null: null,
      undefined: undefined,
    });
    expect(actual).toEqual([
      '.number',
      '.boolean',
      '.string',
      '.function',
      '.null',
      '.undefined',
    ]);
  });

  it('supports simple array values', () => {
    const actual = collectKeys([
      { alpha: true },
      { alpha: true },
      { beta: true },
    ]);
    expect(actual).toEqual(['.alpha', '.alpha', '.beta']);
  });

  it('supports nested object values', () => {
    const actual = collectKeys([
      {
        inner: {
          foo: 'bar',
        },
      },
    ]);
    expect(actual).toEqual(['.inner.foo']);
  });

  it('supports nested array values', () => {
    const actual = collectKeys([[{ foo: 'bar' }], { bar: 'foo' }]);
    expect(actual).toEqual(['.foo', '.bar']);
  });

  it('allows a custom key prefix', () => {
    const actual = collectKeys(
      {
        foo: 1,
      },
      'root'
    );
    expect(actual).toEqual(['root.foo']);
  });
});
