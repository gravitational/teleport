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

import { getFormattedDate } from './date';

test('getFormattedDate', async () => {
  expect(getFormattedDate(null)).toBe('');
  expect(getFormattedDate(new Date('0001-01-01T00:00:00Z'))).toBe('');
  expect(getFormattedDate(undefined)).toBe('');
  expect(getFormattedDate(new Date(null))).toBe('');
  expect(getFormattedDate(new Date('2023-08-24T17:48:15.78579Z'))).toBe(
    '08/24/2023'
  );
});
