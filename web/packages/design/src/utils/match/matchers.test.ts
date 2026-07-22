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

import { dateMatcher, dateTimeMatcher } from './matchers';

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
