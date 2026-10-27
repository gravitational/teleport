/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { addHours, isAfter, isSameDay } from 'date-fns';

import { displayTimeWithTimeZone } from 'design/datetime';
import { Option } from 'shared/components/Select';

import { TimeOption } from '../../Shared/types';
import { getFormattedDurationTxt } from '../../Shared/utils';

// Preset hour options for the time option dropdowns.
export const presetHours = [1, 2, 3, 4, 6, 8, 12, 18];
// Preset day options for the time option dropdowns.
export const presetDays = [1, 2, 3, 4, 5, 6, 7];
const HourInMs = 60 * 60 * 1000;
// Keep this aligned with services.MaxScheduledStartDelay, which the server
// uses to reject starts more than 30 days in the future.
const SCHEDULED_START_LIMIT_MS = 30 * 24 * HourInMs;
const TIME_OPTION_INTERVAL_MINUTES = 30;

export function makeAccessEndOption(start: Date, end: Date): Option<number> {
  return {
    value: end.getTime(),
    label: getFormattedDurationTxt({ start, end }),
  };
}

export function getPendingRequestDurationOptions(
  accessRequestCreated: Date,
  maxDuration: number
): Option<number>[] {
  const createdDate = new Date(accessRequestCreated);

  // Backend limits max pending date to be 1 week.
  // MaxDuration can get greater than 1 week.
  // Pick the smaller of the two.
  let maxPendingDate = new Date(maxDuration);
  const maxPendingDay = presetDays.length - 1;
  const possiblySmallerMaxPendingDate = addHours(
    createdDate,
    presetDays[maxPendingDay] * 24
  );
  if (maxPendingDate.getTime() > possiblySmallerMaxPendingDate.getTime()) {
    maxPendingDate = possiblySmallerMaxPendingDate;
  }

  const createdTimestamp = createdDate.getTime();
  const pendingTimestamp = maxPendingDate.getTime();

  let durationOpts: Option<number>[] = [];

  const totalHoursDiff = (pendingTimestamp - createdTimestamp) / HourInMs;

  // If there is less than an hour available, return
  // it as max duration as the only option.
  if (totalHoursDiff <= 1) {
    durationOpts.push({
      value: pendingTimestamp,
      label: getFormattedDurationTxt({
        start: createdDate,
        end: maxPendingDate,
      }),
    });
    return durationOpts;
  }

  // Add preset hour options up to maximum allowed.
  for (const hour of presetHours) {
    const updatedDateTime = addHours(createdDate, hour);
    if (isAfter(updatedDateTime, maxPendingDate)) {
      break;
    }
    durationOpts.push({
      value: updatedDateTime.getTime(),
      label: getFormattedDurationTxt({
        start: createdDate,
        end: updatedDateTime,
      }),
    });
  }

  // Add preset days up to maximum allowed.
  if (totalHoursDiff >= 24) {
    for (const day of presetDays) {
      const updatedEndDate = addHours(createdDate, day * 24);
      if (isAfter(updatedEndDate, maxPendingDate)) {
        break;
      }
      durationOpts.push({
        value: updatedEndDate.getTime(),
        label: getFormattedDurationTxt({
          start: createdDate,
          end: updatedEndDate,
        }),
      });
    }
  }

  const lastDurationOption = durationOpts[durationOpts.length - 1];
  if (pendingTimestamp > lastDurationOption.value) {
    durationOpts.push({
      value: pendingTimestamp,
      label: getFormattedDurationTxt({
        start: createdDate,
        end: maxPendingDate,
      }),
    });
  }

  return durationOpts;
}

/**
 * Backend expects the maxDuration field to be set to some value
 * which on dry run's will get overwritten to whatever the backend
 * defualt max maxDuration is. Leaving maxDuration field undefined
 * for dry run's, for some reason gets respected.
 */
export function getDryRunMaxDuration() {
  const sevenDaysInMs = 1000 * 60 * 60 * 24 * 7;
  return new Date(Date.now() + sevenDaysInMs);
}

export function getScheduledStartLimit(now = new Date()): Date {
  return new Date(now.getTime() + SCHEDULED_START_LIMIT_MS);
}

export function getScheduledStartTimeOptions(
  selectedDate: Date,
  now = new Date()
): TimeOption[] {
  return getTimeOptionsWithinLimits(
    selectedDate,
    now,
    getScheduledStartLimit(now)
  );
}

export function getPreferredTimeOption(
  preferredTime: Date,
  options: TimeOption[]
): TimeOption | undefined {
  return (
    options.find(
      option =>
        option.value.getHours() === preferredTime.getHours() &&
        option.value.getMinutes() === preferredTime.getMinutes()
    ) || options[0]
  );
}

export function getDefaultScheduledStartTimeOption(
  selectedDate: Date,
  now = new Date()
): TimeOption | undefined {
  const preferredStart = new Date(selectedDate);
  preferredStart.setHours(9, 0, 0, 0);
  return getPreferredTimeOption(
    preferredStart,
    getScheduledStartTimeOptions(selectedDate, now)
  );
}

export function getScheduledEndTimeOptions(
  selectedDate: Date,
  start: Date,
  maxAllowedDuration: number
): TimeOption[] {
  return getTimeOptionsWithinLimits(
    selectedDate,
    start,
    new Date(start.getTime() + maxAllowedDuration)
  );
}

// Returns 30-minute options after the minimum and through the maximum.
// Adds the exact maximum when the last 30-minute option is earlier.
function getTimeOptionsWithinLimits(
  selectedDate: Date,
  minimum: Date,
  maximum: Date
): TimeOption[] {
  const options: TimeOption[] = [];
  for (
    let minutes = 0;
    minutes < 24 * 60;
    minutes += TIME_OPTION_INTERVAL_MINUTES
  ) {
    const option = new Date(selectedDate);
    option.setHours(Math.floor(minutes / 60), minutes % 60, 0, 0);
    if (
      option.getTime() <= minimum.getTime() ||
      option.getTime() > maximum.getTime()
    ) {
      continue;
    }
    options.push({
      value: option,
      label: displayTimeWithTimeZone(option),
    });
  }

  if (
    isSameDay(selectedDate, maximum) &&
    maximum.getTime() > minimum.getTime() &&
    !options.some(option => option.value.getTime() === maximum.getTime())
  ) {
    options.push({
      value: maximum,
      label: displayTimeWithTimeZone(maximum),
    });
  }

  return options.sort(
    (left, right) => left.value.getTime() - right.value.getTime()
  );
}
