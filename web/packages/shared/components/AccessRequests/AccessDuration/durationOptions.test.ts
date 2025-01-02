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

import { addDays, addHours, addWeeks } from 'date-fns';

import {
  DurationOption,
  getDurationOptionIndexClosestToOneWeek,
} from './durationOptions';

describe('getDurationOptionIndexClosestToOneWeek', () => {
  const beginDate = new Date('2024-02-10T03:00:00.000000Z');
  jest.useFakeTimers().setSystemTime(beginDate);

  const durationOpts: DurationOption[] = [
    { value: beginDate.getTime(), label: '' }, // earliest date
    { value: addHours(beginDate, 3).getTime(), label: '' },
    { value: addHours(beginDate, 6).getTime(), label: '' },
    { value: addHours(beginDate, 9).getTime(), label: '' },
    { value: addDays(beginDate, 3).getTime(), label: '' },
    { value: addDays(beginDate, 7).getTime(), label: '' }, //  one week
    { value: addDays(beginDate, 8).getTime(), label: '' },
    { value: addDays(beginDate, 10).getTime(), label: '' },
    { value: addWeeks(beginDate, 2).getTime(), label: '' }, // two week
  ];

  const lastDurationIndex = durationOpts.length - 1;

  test('one week from selected date, is greater than value from last index, returns the last index', () => {
    const startDate = addDays(beginDate, 10);

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate // 1 week from startDate is 17 days, past 2 weeks.
    );
    expect(index).toBe(lastDurationIndex);
  });

  test('one week from selected date, is equal to the value from last index, returns the last index', () => {
    const startDate = addWeeks(beginDate, 1);

    // Ensure the expected option is what we expect.
    expect(durationOpts[lastDurationIndex].value).toBe(
      addWeeks(startDate, 1).getTime()
    );

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate // 1 week from start date is exactly 2 weeks
    );
    expect(index).toBe(lastDurationIndex);
  });

  test('one week from selected date, is less than the last index, returns the index equal to one week', () => {
    const startDate = beginDate;
    const expectedIndex = 5;

    // Ensure the expected option is what we expect.
    expect(durationOpts[expectedIndex].value).toBe(
      addWeeks(beginDate, 1).getTime() // 1 week from start date is exactly 1 week
    );

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate
    );
    expect(index).toBe(expectedIndex);
  });

  test('one week from selected date, is less than the last index, returns the index closest but no greater than one week', () => {
    const startDate = addDays(beginDate, 3);
    const expectedIndex = 7;

    // Ensure the expected option is what we expect.
    expect(durationOpts[expectedIndex].value).toBe(
      addDays(beginDate, 10).getTime() // 1 week from start date is day 10
    );

    const index = getDurationOptionIndexClosestToOneWeek(
      durationOpts,
      startDate
    );
    expect(index).toBe(expectedIndex);
  });
});
