/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { act, renderHook } from '@testing-library/react';
import { addDays, addHours } from 'date-fns';

import { dryRunResponse } from 'shared/components/AccessRequests/fixtures';
import { AccessRequest } from 'shared/services/accessRequests';

import { useSpecifiableFields } from './useSpecifiableFields';

test('scheduled state defaults to eight hours or the exact duration cap', () => {
  // Setup scheduled dry runs with a ten-day limit and a cap under eight hours.
  const start = new Date('2026-11-01T10:00:00Z');
  const tenDayRequest = makeScheduledDryRun(start, 10 * 24 * 60 * 60 * 1000);
  const cappedRequest = makeScheduledDryRun(
    start,
    5 * 60 * 60 * 1000 + 15 * 60 * 1000
  );
  const { result } = renderHook(() => useSpecifiableFields());

  // Execute.
  act(() => result.current.onDryRunChange(tenDayRequest));

  // Assert the ten-day request defaults to eight hours and includes its cap.
  expect(result.current.maxDuration.value).toBe(addHours(start, 8).getTime());
  expect(result.current.maxDurationOptions.at(-1).value).toBe(
    addDays(start, 10).getTime()
  );

  // Execute the capped case.
  act(() => result.current.onDryRunChange(cappedRequest));

  // Assert the exact cap is selected and available.
  expect(result.current.maxDuration.value).toBe(
    cappedRequest.expires.getTime()
  );
  expect(result.current.maxDurationOptions.at(-1)).toEqual(
    result.current.maxDuration
  );
});

test('scheduled state preserves duration across layout and start changes', () => {
  // Setup a scheduled request and a future start.
  const start = new Date('2026-11-01T10:00:00Z');
  const dryRun = makeScheduledDryRun(start, 10 * 24 * 60 * 60 * 1000);
  const futureStart = addDays(start, 20);
  const { result } = renderHook(() => useSpecifiableFields());
  act(() => result.current.onDryRunChange(dryRun));
  const threeHourEnd = addHours(start, 3);

  // Execute an immediate duration change and switch to a future start.
  act(() =>
    result.current.onMaxDurationChange({
      value: threeHourEnd.getTime(),
      label: '3 hours',
    })
  );
  act(() => result.current.onStartTimeChange(futureStart));

  // Assert the duration is preserved independently of the start delay.
  expect(result.current.startTime).toEqual(futureStart);
  expect(result.current.maxDuration.value).toBe(
    addHours(futureStart, 3).getTime()
  );

  // Execute an end change and then return to the immediate layout.
  act(() =>
    result.current.onMaxDurationChange({
      value: addHours(futureStart, 5).getTime(),
      label: '5 hours',
    })
  );
  act(() => result.current.onStartTimeChange(null));

  // Assert the future-derived duration and exact option are preserved.
  expect(result.current.startTime).toBeNull();
  expect(result.current.maxDuration.value).toBe(addHours(start, 5).getTime());
  expect(result.current.maxDurationOptions).toContainEqual(
    result.current.maxDuration
  );
});

function makeScheduledDryRun(start: Date, duration: number): AccessRequest {
  const end = new Date(start.getTime() + duration);
  return {
    ...dryRunResponse,
    created: new Date(start),
    expires: end,
    maxDuration: end,
    requestTTL: end,
    timing: {
      scheduled: {
        start: new Date(start),
        duration,
      },
    },
  };
}
