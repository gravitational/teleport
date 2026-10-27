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

import {
  dateTimeShortFormat,
  displayDate,
  displayDateTime,
  displayDateTimeWithTimeZone,
  formatTimeZone,
  formatTimeZoneOffset,
} from './format';

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

test('formatTimeZone uses the offset and abbreviation at the displayed instant', () => {
  // Setup winter and summer instants in New York.
  const winter = new Date('2026-01-15T12:00:00Z');
  const summer = new Date('2026-07-15T12:00:00Z');

  // Execute.
  const winterZone = formatTimeZone(winter, 'America/New_York');
  const summerZone = formatTimeZone(summer, 'America/New_York');

  // Assert each instant uses its applicable offset and abbreviation.
  expect(winterZone).toBe('UTC-05:00 EST');
  expect(summerZone).toBe('UTC-04:00 EDT');
});

test('formatTimeZoneOffset uses a compact UTC offset', () => {
  const date = new Date('2026-07-15T12:00:00Z');

  expect(formatTimeZoneOffset(date, 'America/Edmonton')).toBe('UTC-6');
});

test('displayDateTimeWithTimeZone formats the browser-local instant', () => {
  // Setup a summer instant in Edmonton.
  const date = new Date('2026-07-15T12:00:00Z');

  // Execute.
  const output = displayDateTimeWithTimeZone(date, 'America/Edmonton');

  // Assert the output includes local time, offset, and abbreviation.
  expect(output).toBe('Jul 15, 2026, 6:00 AM (UTC-06:00 MDT)');
});
