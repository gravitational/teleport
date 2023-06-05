/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import ls from './localStorage';
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
    localStorage.setItem('key2', 'val2');
    localStorage.setItem(KeysEnum.SHOW_ASSIST_POPUP, '');
    localStorage.setItem('key3', 'val3');
    localStorage.setItem(KeysEnum.LAST_ACTIVE, '');

    expect(localStorage).toHaveLength(5);

    ls.clear();
    expect(localStorage).toHaveLength(1);
    expect(localStorage.key(0)).toBe(KeysEnum.SHOW_ASSIST_POPUP);
  });

  test('delete on empty length is not an error', () => {
    expect(localStorage).toHaveLength(0);
    ls.clear();
    expect(localStorage).toHaveLength(0);
  });
});
