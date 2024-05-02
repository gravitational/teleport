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

import { AccessRequest } from 'shared/services/accessRequests';

import { getTimeOptions } from './timeOptions';

test('same day limit produces options with a min and a max', () => {
  // Same days but with different time.
  const created = new Date('2024-02-16T03:00:08.156944Z'); // 02/16
  const maxDuration = new Date('2024-02-16T06:45:08.156944Z'); // 02/16

  const selectedDate = new Date(created);

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const times = getTimeOptions(selectedDate, mockAccessRequest);

  // min option should be the same as created hours
  expect(times[0].value).toStrictEqual(new Date('2024-02-16T03:00:00.000Z'));

  // max option should be the same as max duration date time - 1 hr
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-16T05:00:00.000Z')
  );
});

test('in between day selection produces every options (no limit)', () => {
  const created = new Date('2024-02-16T03:00:08.156944Z'); // 02/16
  const maxDuration = new Date('2024-02-18T06:45:08.156944Z'); // 02/18

  // The in between date, 02/17
  const selectedDate = new Date('2024-02-17T03:00:08.156944Z');

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const times = getTimeOptions(selectedDate, mockAccessRequest);

  // min option defaults to the earliest time available for the day
  expect(times[0].value).toStrictEqual(new Date('2024-02-17T00:00:00.000Z'));

  // max option defaults to the latest time available for the day
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-17T23:00:00.000Z')
  );
});

test('first day selection produces options with only a min limit', () => {
  const created = new Date('2024-02-16T03:00:08.156944Z'); // 02/16
  const maxDuration = new Date('2024-02-18T06:45:08.156944Z'); // 02/18

  // The first day is the created date.
  const selectedDate = new Date(created);

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const times = getTimeOptions(selectedDate, mockAccessRequest);

  // min option is limited to the created date time
  expect(times[0].value).toStrictEqual(new Date('2024-02-16T03:00:00.000Z'));

  // max option defaults to the latest time available for the day
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-16T23:00:00.000Z')
  );
});

test('last day selection produces option with only a max limit', () => {
  const created = new Date('2024-02-16T03:00:08.156944Z'); // 02/16
  const maxDuration = new Date('2024-02-18T06:45:08.156944Z'); // 02/18

  // The last day is the max duration date
  const selectedDate = new Date(maxDuration);

  jest.useFakeTimers().setSystemTime(created);
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const times = getTimeOptions(selectedDate, mockAccessRequest);

  // min option defaults to the earliest time available for the day
  expect(times[0].value).toStrictEqual(new Date('2024-02-18T00:00:00.000Z'));

  // max option is limited to the max duration time - 1.
  expect(times[times.length - 1].value).toStrictEqual(
    new Date('2024-02-18T05:00:00.000Z')
  );
});

test('on reviewing mode, start time options should start from current date time', () => {
  const current = new Date('2024-02-16T11:00:08.156944Z'); // 11 pm
  jest.useFakeTimers().setSystemTime(current);

  const created = new Date('2024-02-16T03:00:08.156944Z'); // 3pm
  const maxDuration = new Date('2024-02-18T06:45:08.156944Z');
  mockAccessRequest.created = created;
  mockAccessRequest.maxDuration = maxDuration;

  const selectedDate = new Date(created);

  const times = getTimeOptions(
    selectedDate,
    mockAccessRequest,
    true /* reviewing */
  );

  // min option defaults to "current" date & hour.
  expect(times[0].value).toStrictEqual(new Date('2024-02-16T11:00:00.000Z'));
});

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
