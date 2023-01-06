/**
 * Copyright 2022 Gravitational, Inc.
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

import { matchLabels } from './DownloadScript';

const dbLabels = [
  { name: 'os', value: 'mac' },
  { name: 'env', value: 'prod' },
  { name: 'tag', value: 'v11.0.0' },
];

const agentLabels = [
  { name: 'a', value: 'b' },
  { name: 'aa', value: 'bb' },
  { name: 'tag', value: 'v11.0.0' }, // match
  { name: 'aaa', value: 'bbb' },
];

describe('matchLabels', () => {
  test.each`
    desc                            | agentLabels                           | expected
    ${'match with multi elements'}  | ${agentLabels}                        | ${true}
    ${'match by both fields'}       | ${[{ name: 'os', value: 'mac' }]}     | ${true}
    ${'match by asteriks'}          | ${[{ name: '*', value: '*' }]}        | ${true}
    ${'match by value'}             | ${[{ name: '*', value: 'mac' }]}      | ${true}
    ${'match by key'}               | ${[{ name: 'os', value: '*' }]}       | ${true}
    ${'no match'}                   | ${[{ name: 'os', value: 'windows' }]} | ${false}
    ${'no match with any key'}      | ${[{ name: '*', value: 'windows' }]}  | ${false}
    ${'no match with any val'}      | ${[{ name: 'id', value: '*' }]}       | ${false}
    ${'no match with empty list'}   | ${[]}                                 | ${false}
    ${'no match with empty fields'} | ${[{ name: '', value: '' }]}          | ${false}
  `('$desc', ({ agentLabels, expected }) => {
    expect(matchLabels(dbLabels, agentLabels)).toEqual(expected);
  });
});
