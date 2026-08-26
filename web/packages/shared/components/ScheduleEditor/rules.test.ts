/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { timezoneOptions, weekdayOptions } from './const';
import { validSchedule, validShift } from './rules';
import { Schedule, Shift, Weekday } from './types';

describe('validShift', () => {
  const testCases: {
    name: string;
    shift: Shift;
    valid: boolean;
    message?: string;
  }[] = [
    {
      name: 'valid shift',
      shift: {
        startTime: { value: '00:00', label: '12:00AM' },
        endTime: { value: '23:59', label: '11:59PM' },
      },
      valid: true,
    },
    {
      name: 'invalid time value',
      shift: {
        startTime: { value: '00:00', label: '12:00AM' },
        endTime: { value: '24:00', label: '12:00PM' },
      },
      valid: false,
      message: 'invalid time',
    },
    {
      name: 'same start and end time',
      shift: {
        startTime: { value: '00:00', label: '12:00AM' },
        endTime: { value: '00:00', label: '12:00AM' },
      },
      valid: false,
      message: 'start time must be before end time',
    },
  ];
  test.each(testCases)('$name', tc => {
    const result = validShift(tc.shift)();
    expect(result.valid).toEqual(tc.valid);
    expect(result.message).toEqual(tc.message);
  });
});

describe('validSchedule', () => {
  const testCases: {
    name: string;
    schedule: Schedule;
    valid: boolean;
    message?: string;
  }[] = [
    {
      name: 'valid schedule',
      schedule: {
        name: 'test',
        timezone: timezoneOptions[0],
        shifts: {
          ...newShifts(),
          Monday: {
            startTime: { value: '00:00', label: '12:00AM' },
            endTime: { value: '23:59', label: '11:59PM' },
          },
          Tuesday: {
            startTime: { value: '00:00', label: '12:00AM' },
            endTime: { value: '23:59', label: '11:59PM' },
          },
        },
      },
      valid: true,
    },
    {
      name: 'missing shifts',
      schedule: {
        name: 'test',
        timezone: timezoneOptions[0],
        shifts: newShifts(),
      },
      valid: false,
      message: 'At least one shift is required.',
    },
  ];
  test.each(testCases)('$name', tc => {
    const result = validSchedule(tc.schedule)();
    expect(result.valid).toEqual(tc.valid);
    expect(result.message).toEqual(tc.message);
  });
});

function newShifts(): Record<Weekday, Shift | null> {
  return weekdayOptions.reduce(
    (shifts, weekday) => {
      shifts[weekday.value] = null;
      return shifts;
    },
    {} as Record<Weekday, Shift | null>
  );
}
