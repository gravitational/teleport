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

import { TimezoneOptions } from './const';
import { validSchedule, validShift } from './rules';
import { Schedule, Shift } from './types';

describe('validShift', () => {
  const testCases: {
    name: string;
    shift: Shift;
    valid: boolean;
    contains: string;
  }[] = [
    {
      name: 'valid shift',
      shift: {
        startTime: { value: '00:00', label: '00:00' },
        endTime: { value: '23:59', label: '23:59' },
      },
      valid: true,
      contains: '',
    },
    {
      name: 'invalid time value',
      shift: {
        startTime: { value: '00:00', label: '00:00' },
        endTime: { value: '24:00', label: '24:00' },
      },
      valid: false,
      contains: 'invalid time',
    },
    {
      name: 'same start and end time',
      shift: {
        startTime: { value: '00:00', label: '00:00' },
        endTime: { value: '00:00', label: '00:00' },
      },
      valid: false,
      contains: 'start time must be before end time',
    },
  ];
  test.each(testCases)('$name', tc => {
    const result = validShift(tc.shift)();
    expect(result.valid).toEqual(tc.valid);
    if (tc.contains) {
      expect(result.message).not.toBeNull();
      expect(result.message).toContain(tc.contains);
    }
  });
});

describe('validSchedule', () => {
  const testCases: {
    name: string;
    schedule: Schedule;
    valid: boolean;
    contains: string;
  }[] = [
    {
      name: 'valid schedule',
      schedule: {
        name: 'test',
        timezone: TimezoneOptions[0],
        shifts: {
          Monday: {
            startTime: { value: '00:00', label: '00:00' },
            endTime: { value: '23:59', label: '23:59' },
          },
          Tuesday: {
            startTime: { value: '00:00', label: '00:00' },
            endTime: { value: '23:59', label: '23:59' },
          },
        },
      },
      valid: true,
      contains: '',
    },
    {
      name: 'missing shifts',
      schedule: {
        name: 'test',
        timezone: TimezoneOptions[0],
        shifts: {},
      },
      valid: false,
      contains: 'At least one shift is required.',
    },
  ];
  test.each(testCases)('$name', tc => {
    const result = validSchedule(tc.schedule)();
    expect(result.valid).toEqual(tc.valid);
    if (tc.contains) {
      expect(result.message).not.toBeNull();
      expect(result.message).toContain(tc.contains);
    }
  });
});
