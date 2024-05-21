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

import { displayDate, displayDateTime, dateTimeShortFormat } from './loc';

const testDate = new Date('2022-01-28T16:00:44.309Z');

test('displayDate', () => {
  const output = displayDate(testDate);

  expect(output).toBe('2022-01-28');
});

test('displayDateTime', () => {
  const output = displayDateTime(testDate);

  expect(output).toBe('2022-01-28 16:00:44');
});

test('dateTimeShortFormat', () => {
  expect(dateTimeShortFormat(testDate)).toEqual('4:00 PM');
});
