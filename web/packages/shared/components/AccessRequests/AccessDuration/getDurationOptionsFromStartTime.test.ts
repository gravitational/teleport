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

import { addDays } from 'date-fns';

import { Option } from 'shared/components/Select';
import { AccessRequest } from 'shared/services/accessRequests';

import {
  getDurationOptionsFromStartTime,
  PRESET_DAYS,
  PRESET_HOURS,
} from './durationOptions';

test('duration difference is less than an hour returns only the max duration', () => {
  const created = new Date('2024-02-16T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-16T03:45:00.156944Z');
  const selectedDate = new Date(created);

  selectedDate.setHours(
    3 /* hours */,
    0 /* minutes */,
    0 /* sec */,
    0 /* ms */
  );

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(selectedDate, mockAccessRequest);

  // Only one option, the max duration.
  expect(opts).toHaveLength(1);
  expect(opts[0].value).toBe(maxDuration.getTime());
  expect(opts[0].label).toBe('45 minutes');
});

test('duration difference is 1hr 30min, returns an hour option and the max duration', () => {
  const created = new Date('2024-02-16T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-16T04:30:00.156944Z');
  const selectedDate = new Date(created);

  selectedDate.setHours(
    3 /* hours */,
    0 /* minutes */,
    0 /* sec */,
    0 /* ms */
  );

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(selectedDate, mockAccessRequest);

  expect(opts).toHaveLength(2);

  // one preset hour
  expect(opts[0].value).toBe(
    new Date(selectedDate).setHours(
      selectedDate.getHours() + PRESET_HOURS[0],
      0,
      0,
      0
    )
  );
  expect(opts[0].label).toBe('1 hour');

  // max duration
  expect(opts[1].value).toBe(maxDuration.getTime());
  expect(opts[1].label).toBe('1 hour 30 minutes');
});

test('defining all preset hours', () => {
  expect(PRESET_HOURS).toHaveLength(8);

  const created = new Date('2024-02-16T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-16T21:00:00.156944Z');
  const selectedDate = new Date(created);

  selectedDate.setHours(
    3 /* hours */,
    0 /* minutes */,
    0 /* sec */,
    0 /* ms */
  );

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(selectedDate, mockAccessRequest);

  expect(opts).toHaveLength(PRESET_HOURS.length);
  testPresetHours(opts, selectedDate);
});

test('defining all preset days + preset hours + maxest duration', () => {
  expect(PRESET_HOURS).toHaveLength(8);
  expect(PRESET_DAYS).toHaveLength(14);

  const created = new Date('2024-02-11T03:00:00.156944Z');
  const maxDuration = new Date('2024-02-25T03:30:00.156944Z');
  const selectedDate = new Date(created);
  selectedDate.setHours(
    3 /* hours */,
    0 /* minutes */,
    0 /* sec */,
    0 /* ms */
  );

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const opts = getDurationOptionsFromStartTime(selectedDate, mockAccessRequest);

  expect(opts).toHaveLength(PRESET_HOURS.length + PRESET_DAYS.length + 1);
  testPresetHours(opts, selectedDate);

  const startDateTime = new Date(created);
  startDateTime.setHours(selectedDate.getHours(), 0, 0, 0);
  for (let i = 0; i < PRESET_DAYS.length; i += 1) {
    const optionIndex = i + PRESET_HOURS.length;
    expect(opts[optionIndex].value).toBe(
      addDays(startDateTime, PRESET_DAYS[i]).getTime()
    );
    const dayTxt = i ? 'days' : 'day';
    expect(opts[optionIndex].label).toBe(`${PRESET_DAYS[i]} ${dayTxt}`);
  }

  // Test maxest duration.
  expect(opts[opts.length - 1].value).toBe(maxDuration.getTime());
  expect(opts[opts.length - 1].label).toBe('14 days 30 minutes');
});

function testPresetHours(opts: Option<number>[], selectedDate: Date) {
  // one preset hour
  for (let i = 0; i < PRESET_HOURS.length; i += 1) {
    expect(opts[i].value).toBe(
      new Date(selectedDate).setHours(
        selectedDate.getHours() + PRESET_HOURS[i],
        0, // min
        0, // sec
        0 // ms
      )
    );
    const hourTxt = i ? 'hours' : 'hour';
    expect(opts[i].label).toBe(`${PRESET_HOURS[i]} ${hourTxt}`);
  }
}

const mockAccessRequest: AccessRequest = {
  id: '31a711f6-f53a-4d61-baae-2c3c8d9a3fd9',
  state: 'PENDING',
  resolveReason: '',
  requestReason: '',
  user: 'lisa',
  roles: ['@teleport-access-approver'],
  created: new Date('2024-02-16T03:00:08.156944Z'),
  createdDuration: '',
  expires: new Date('2024-02-19T03:00:08.157365Z'),
  expiresDuration: '',
  maxDuration: new Date('2024-02-19T03:00:08.157365Z'),
  maxDurationText: '',
  requestTTL: new Date('2024-02-16T04:00:08.157365Z'),
  requestTTLDuration: '',
  sessionTTL: new Date('2024-02-16T12:11:46.99997Z'),
  sessionTTLDuration: '',
  reviews: [],
  reviewers: [],
  thresholdNames: ['default'],
  resources: [],
  assumeStartTime: null,
};
