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

import { addDays, addHours } from 'date-fns';

import { Option } from 'shared/components/Select';

import {
  getPendingRequestDurationOptions,
  presetDays,
  presetHours,
} from './utils';

test('duration is less than 1 hour', () => {
  const created = new Date('2024-02-16T03:00:00.000000Z');
  const maxDuration = new Date('2024-02-16T03:45:00.000000Z');

  jest.useFakeTimers().setSystemTime(created);
  const opts = getPendingRequestDurationOptions(created, maxDuration.getTime());

  // Only one option, the max duration.
  expect(opts).toHaveLength(1);
  expect(opts[0].value).toBe(maxDuration.getTime());
  expect(opts[0].label).toBe('45 minutes');
});

test('duration is a mix of some preset hours and the max duration', () => {
  const created = new Date('2024-02-16T03:00:00.000000Z');
  const maxDuration = new Date('2024-02-16T05:45:00.000000Z');

  jest.useFakeTimers().setSystemTime(created);
  const opts = getPendingRequestDurationOptions(created, maxDuration.getTime());

  // Only one option, the max duration.
  expect(opts).toHaveLength(3);
  expect(opts[0].value).toBe(addHours(created, 1).getTime());
  expect(opts[0].label).toBe('1 hour');

  expect(opts[1].value).toBe(addHours(created, 2).getTime());
  expect(opts[1].label).toBe('2 hours');

  expect(opts[2].value).toBe(maxDuration.getTime());
  expect(opts[2].label).toBe('2 hours 45 minutes');
});

test('defining all preset hours', () => {
  expect(presetHours).toHaveLength(8);

  const created = new Date('2024-02-16T03:00:00.000000Z');
  const maxDuration = new Date('2024-02-16T21:00:00.000000Z');

  jest.useFakeTimers().setSystemTime(created);
  const opts = getPendingRequestDurationOptions(created, maxDuration.getTime());

  expect(opts).toHaveLength(presetHours.length);
  testPresetHours(opts, created);
});

test('defining all preset days + preset hours + maxest', () => {
  expect(presetHours).toHaveLength(8);
  expect(presetDays).toHaveLength(7);

  const created = new Date('2024-02-16T03:00:00.000000Z');
  const maxDuration = new Date('2024-02-27T03:30:00.000000Z');

  jest.useFakeTimers().setSystemTime(created);
  const opts = getPendingRequestDurationOptions(created, maxDuration.getTime());

  expect(opts).toHaveLength(presetHours.length + presetDays.length);
  testPresetHours(opts, created);

  for (let i = 0; i < presetDays.length; i += 1) {
    const optionIndex = i + presetHours.length;
    const dayTxt = i ? 'days' : 'day';

    if (i == presetDays.length - 1) {
      break;
    }
    expect(opts[optionIndex].label).toBe(`${presetDays[i]} ${dayTxt}`);
    expect(opts[optionIndex].value).toBe(
      addDays(created, presetDays[i]).getTime()
    );
  }

  // Test maxest duration.
  expect(opts[opts.length - 1].value).toBe(addDays(created, 7).getTime());
  expect(opts[opts.length - 1].label).toBe('7 days');
});

function testPresetHours(opts: Option<number>[], createdDate: Date) {
  // one preset hour
  for (let i = 0; i < presetHours.length; i += 1) {
    const addedDate = addHours(createdDate, presetHours[i]);
    const hourTxt = i ? 'hours' : 'hour';

    expect(opts[i].label).toBe(`${presetHours[i]} ${hourTxt}`);
    expect(opts[i].value).toBe(addedDate.getTime());
  }
}
