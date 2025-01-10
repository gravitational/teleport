/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { generateTimeDropdown } from './timeOptions';

test('no limit, 60 min increment, generates every options', () => {
  const startDate = new Date('2024-02-16T03:00:08.156944Z');
  jest.useFakeTimers().setSystemTime(startDate);

  const times = generateTimeDropdown({ startDate }, 60);

  // 24 hours total, since it's hourly increment
  expect(times).toHaveLength(24);

  // Earliest time
  expect(times[0].label).toBe('12:00 AM');
  expect(times[0].value).toStrictEqual(new Date('2024-02-16T00:00:00.000Z'));

  // Middle
  expect(times[times.length / 2].label).toBe('12:00 PM');
  expect(times[times.length / 2].value).toStrictEqual(
    new Date('2024-02-16T12:00:00.000Z')
  );

  // Last
  expect(times[times.length - 1].label).toBe('11:00 PM');
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-16T23:00:00.000Z')
  );
});

test('with min limit, 60 min increment, generates options beginnigng from min', () => {
  const startDate = new Date('2024-02-16T20:00:08.156944Z');
  jest.useFakeTimers().setSystemTime(startDate);

  const times = generateTimeDropdown(
    { startDate, minTimestamp: startDate.getTime() },
    60
  );

  expect(times).toHaveLength(4); // 8PM - 11PM

  // Earliest time available is the same time as the start date.
  expect(times[0].label).toBe('8:00 PM');
  expect(times[0].value).toStrictEqual(new Date('2024-02-16T20:00:00.000Z'));

  // Last time of day.
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-16T23:00:00.000Z')
  );
});

test('with both a min and a max limit, 60 min increment, generates options within the min/max range', () => {
  const startDate = new Date('2024-02-16T08:00:08.156944Z');
  const endDate = new Date('2024-02-16T10:00:08.156944Z');
  jest.useFakeTimers().setSystemTime(startDate);

  const times = generateTimeDropdown(
    {
      startDate,
      minTimestamp: startDate.getTime(),
      maxTimestamp: endDate.getTime(),
    },
    60
  );

  expect(times).toHaveLength(3); // 8AM - 10AM

  expect(times[0].value).toStrictEqual(new Date('2024-02-16T08:00:00.000Z'));

  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-16T10:00:00.000Z')
  );
});

test('with max limit, 60 min increment, generates options ending with max limit', () => {
  const startDate = new Date('2024-02-16T08:00:08.156944Z');
  jest.useFakeTimers().setSystemTime(startDate);

  const times = generateTimeDropdown(
    { startDate, maxTimestamp: startDate.getTime() },
    60
  );

  expect(times).toHaveLength(9); // 12AM - 8AM

  // Earliest time available is the same time as the start date.
  expect(times[0].label).toBe('12:00 AM');
  expect(times[0].value).toStrictEqual(new Date('2024-02-16T00:00:00.000Z'));

  // Last time of day.
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-16T08:00:00.000Z')
  );
});

test('no limit, 15 min increment', () => {
  const startDate = new Date('2024-02-16T03:00:08.156944Z');
  jest.useFakeTimers().setSystemTime(startDate);

  const times = generateTimeDropdown({ startDate }, 15);

  // 24 hours total * 4, 15 min increments.
  expect(times).toHaveLength(24 * 4);

  // Test first quarters
  expect(times[0].label).toBe('12:00 AM');
  expect(times[0].value).toStrictEqual(new Date('2024-02-16T00:00:00.000Z'));
  expect(times[1].label).toBe('12:15 AM');
  expect(times[1].value).toStrictEqual(new Date('2024-02-16T00:15:00.000Z'));
  expect(times[2].label).toBe('12:30 AM');
  expect(times[2].value).toStrictEqual(new Date('2024-02-16T00:30:00.000Z'));
  expect(times[3].label).toBe('12:45 AM');
  expect(times[3].value).toStrictEqual(new Date('2024-02-16T00:45:00.000Z'));

  // Middle
  expect(times[times.length / 2].label).toBe('12:00 PM');
  expect(times[times.length / 2].value).toStrictEqual(
    new Date('2024-02-16T12:00:00.000Z')
  );

  // Last
  expect(times[times.length - 1].label).toBe('11:45 PM');
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-16T23:45:00.000Z')
  );
});
