/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
 *
 * @jest-environment @gravitational/build/jest/jest-environment-time-zone
 * @jest-environment-options {"timeZone": "America/Los_Angeles"}
 */

import { AccessRequest } from 'shared/services/accessRequests';

import {
  getDurationOptionIndexClosestToOneWeek,
  getDurationOptionsFromStartTime,
  PRESET_DAYS,
} from './AccessDuration/durationOptions';
import { dryRunResponse } from './fixtures';
import {
  getPendingRequestDurationOptions,
  presetDays,
} from './NewRequest/RequestCheckout/utils';

const DayInMs = 24 * 60 * 60 * 1000;
// services.requestTTL.
const MaxPendingTtlInMs = 7 * DayInMs;

// Within two weeks of a 2026 America/Los_Angeles transition.
const beforeFallBack = new Date(2026, 9, 29, 10, 0, 0, 0);
const beforeSpringForward = new Date(2026, 2, 2, 10, 0, 0, 0);

function offsetsFrom(options: { value: number }[], start: Date): number[] {
  return options
    .map(option => option.value - start.getTime())
    .filter(offset => offset >= DayInMs);
}

describe.each([
  ['fall back', beforeFallBack],
  ['spring forward', beforeSpringForward],
])('across the %s transition', (_name, created) => {
  // Long enough that the seven-day cap is what limits the pending options.
  const accessEnd = created.getTime() + 14 * DayInMs;

  test('pending day options are whole days, up to the server limit', () => {
    // Execute.
    const opts = getPendingRequestDurationOptions(created, accessEnd);

    // Assert every day option is a whole day and the last is the limit.
    expect(offsetsFrom(opts, created)).toEqual(
      presetDays.map(day => day * DayInMs)
    );
    expect(opts.at(-1).label).toBe('7 days');
    expect(opts.at(-1).value - created.getTime()).toBe(MaxPendingTtlInMs);
  });

  test('the preselected pending TTL stays within the server limit', () => {
    // Setup the options a requester would be offered.
    const opts = getPendingRequestDurationOptions(created, accessEnd);

    // Execute, mirroring preselectPendingRequestTtlOption.
    const selected =
      opts[getDurationOptionIndexClosestToOneWeek(opts, created)];

    // Assert the preselected option is one the server accepts.
    expect(selected.value - created.getTime()).toBe(MaxPendingTtlInMs);
  });

  test('access duration day options are whole days', () => {
    // Setup a request with a 14-day access limit.
    const request: AccessRequest = {
      ...dryRunResponse,
      created,
      maxDuration: new Date(accessEnd),
    };

    // Execute.
    const opts = getDurationOptionsFromStartTime(created, request);

    // Assert each preset day is a whole day.
    expect(offsetsFrom(opts, created)).toEqual(
      PRESET_DAYS.map(day => day * DayInMs)
    );
  });
});
