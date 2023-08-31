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

import { dateTimeMatcher, dateMatcher } from './matchers';

test('dateTimeMatcher should match date times correctly', () => {
  const searchValue = '23:13';

  const dateMatched = new Date('2023-05-04T23:13:55.539Z');
  const dateMatchedISOString = '2023-05-04T23:13:55.539Z';
  const dateNoMatch = new Date('2023-05-04T12:11:00.539Z');
  const dateInvalid = new Date('invalid');

  // Should be `true` because the date matches the searchValue.
  const resultMatched = dateTimeMatcher(['dateTime'])(
    dateMatched,
    searchValue,
    'dateTime'
  );

  // Should be `true` because the ISO string should be converted to `Date` and the date matches the searchValue.
  const resultMatchedISO = dateTimeMatcher(['dateTime'])(
    dateMatchedISOString,
    searchValue,
    'dateTime'
  );

  // Should be `false` because the date does not match the searchValue.
  const resultNoMatch = dateTimeMatcher(['dateTime'])(
    dateNoMatch,
    searchValue,
    'dateTime'
  );

  // Should be `false` because the date is invalid
  const resultInvalid = dateTimeMatcher(['dateTime'])(
    dateInvalid,
    searchValue,
    'dateTime'
  );

  expect(resultMatched).toBe(true);
  expect(resultMatchedISO).toBe(true);
  expect(resultNoMatch).toBe(false);
  expect(resultInvalid).toBe(false);
});

test('dateMatcher should match dates correctly', () => {
  const searchValue = '2023-05-04';

  const dateMatched = new Date('2023-05-04T23:13:55.539Z');
  const dateMatchedISOString = '2023-05-04T23:13:55.539Z';
  const dateNoMatch = new Date('2022-05-04T23:13:55.539Z');
  const dateInvalid = new Date('invalid');

  // Should be `true` because the date matches the searchValue.
  const resultMatched = dateMatcher(['dateTime'])(
    dateMatched,
    searchValue,
    'dateTime'
  );

  // Should be `true` because the ISO string should be converted to `Date` and the date matches the searchValue.
  const resultMatchedISO = dateMatcher(['dateTime'])(
    dateMatchedISOString,
    searchValue,
    'dateTime'
  );

  // Should be `false` because the date does not match the searchValue.
  const resultNoMatch = dateMatcher(['dateTime'])(
    dateNoMatch,
    searchValue,
    'dateTime'
  );

  // Should be `false` because the date is invalid
  const resultInvalid = dateMatcher(['dateTime'])(
    dateInvalid,
    searchValue,
    'dateTime'
  );

  expect(resultMatched).toBe(true);
  expect(resultMatchedISO).toBe(true);
  expect(resultNoMatch).toBe(false);
  expect(resultInvalid).toBe(false);
});
