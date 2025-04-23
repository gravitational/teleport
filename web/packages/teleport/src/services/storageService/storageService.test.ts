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

import { storageService as ls } from './storageService';
import { KeysEnum } from './types';

describe('localStorage', () => {
  afterEach(() => {
    localStorage.clear();
  });

  test('deletes all keys', () => {
    // add a few keys
    localStorage.setItem('key1', 'val1');
    localStorage.setItem('key2', 'val2');
    localStorage.setItem('key3', 'val3');
    expect(localStorage).toHaveLength(3);

    ls.clear();
    expect(localStorage).toHaveLength(0);
  });

  test('does not delete keys under KEEP_LOCALSTORAGE_KEYS_ON_LOGOUT', () => {
    // add a few keys
    localStorage.setItem('key1', 'val1');
    localStorage.setItem(KeysEnum.THEME, '');
    localStorage.setItem('key2', 'val2');
    localStorage.setItem(KeysEnum.LAST_ACTIVE, '');

    expect(localStorage).toHaveLength(4);

    ls.clear();
    expect(localStorage).toHaveLength(1);
    expect(localStorage.key(0)).toBe(KeysEnum.THEME);
  });

  test('delete on empty length is not an error', () => {
    expect(localStorage).toHaveLength(0);
    ls.clear();
    expect(localStorage).toHaveLength(0);
  });

  test('parses JSON configuration values', () => {
    localStorage.setItem('key1', '{"foo": "bar"}');
    localStorage.setItem('key2', 'true');

    expect(ls.getParsedJSONValue('key1', null)).toEqual({ foo: 'bar' });
    expect(ls.getParsedJSONValue('key2', null)).toBe(true);
    expect(ls.getParsedJSONValue('not-a-key', 'default')).toBe('default');
  });
});
