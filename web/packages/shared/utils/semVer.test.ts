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

import { compareSemVers } from './semVer';

test('compareSemVers', () => {
  expect(['3.0.0', '1.0.0', '2.0.0'].sort(compareSemVers)).toEqual([
    '1.0.0',
    '2.0.0',
    '3.0.0',
  ]);

  expect(['3.1.0', '3.2.0', '3.1.1'].sort(compareSemVers)).toEqual([
    '3.1.0',
    '3.1.1',
    '3.2.0',
  ]);

  expect(['10.0.1', '10.0.2', '2.0.0'].sort(compareSemVers)).toEqual([
    '2.0.0',
    '10.0.1',
    '10.0.2',
  ]);

  expect(['10.1.0', '11.1.0', '5.10.10'].sort(compareSemVers)).toEqual([
    '5.10.10',
    '10.1.0',
    '11.1.0',
  ]);
});
