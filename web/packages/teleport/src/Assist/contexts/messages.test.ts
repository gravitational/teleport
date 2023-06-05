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

import { sortLoginsWithRootLoginsLast } from './messages';

describe('assist messages', () => {
  it('should sort logins alphabetically', () => {
    const logins = ['test', 'login1', 'awesome', 'random', 'login2'];

    const sorted = sortLoginsWithRootLoginsLast(logins);

    expect(sorted).toEqual(['awesome', 'login1', 'login2', 'random', 'test']);
  });

  it('should put root logins last', () => {
    const logins = [
      'root',
      'admin',
      'test',
      'login1',
      'awesome',
      'random',
      'login2',
    ];

    const sorted = sortLoginsWithRootLoginsLast(logins);

    expect(sorted).toEqual([
      'awesome',
      'login1',
      'login2',
      'random',
      'test',
      'admin',
      'root',
    ]);
  });
});
