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

import { addDays, isSameDay } from 'date-fns';

import { dateTimeShortFormat } from 'shared/services/loc/loc';

import { AccessRequest } from 'shared/services/accessRequests';

import { TimeOption } from '../Shared/types';

const OneDayInMinutes = 1440; // 24 hours
export const OneWeek = 7;

// DateTimeLimit defines the earliest a time can start
// and the latest time can end.
type DateTimeLimit = {
  minTimestamp?: number;
  maxTimestamp?: number;
  startDate: Date;
};

/**
 * isWithinTimeLimit checks if the current timestamp
 * is within the limits of min and max timestamps.
 * Returns false if current timestamp is out of min/max range.
 */
function isWithinTimeLimit(limit: DateTimeLimit, currentTimestamp: number) {
  if (limit.minTimestamp && limit.maxTimestamp) {
    return (
      currentTimestamp >= limit.minTimestamp &&
      currentTimestamp <= limit.maxTimestamp
    );
  }

  if (limit.minTimestamp) {
    return currentTimestamp >= limit.minTimestamp;
  }

  if (limit.maxTimestamp) {
    return currentTimestamp <= limit.maxTimestamp;
  }

  return true;
}

/**
 * generateTimeDropdown generates time options in format 00:00 AM|PM
 * Time can start as early as 12:00 AM and can be as late as 11:30 PM
 * (if `incrementBy = 30` for example)
 *
 * The range of options is defined by the limit param.
 */
export function generateTimeDropdown(
  limit: DateTimeLimit,
  incrementTimeBy = 60 // default to incrementing time by the hour
) {
  const times: TimeOption[] = [];

  for (let i = 0; i < OneDayInMinutes; i += incrementTimeBy) {
    const militaryHrs = Math.floor(i / 60);
    const minutes = i % 60;

    const currentDate = new Date(limit.startDate);
    if (!isWithinTimeLimit(limit, currentDate.setHours(militaryHrs, minutes))) {
      continue;
    }

    const date = new Date(limit.startDate);
    date.setHours(militaryHrs, minutes, 0 /* sec */, 0 /* ms */);
    times.push({
      label: dateTimeShortFormat(date),
      value: date,
    });
  }

  return times;
}

export function convertStartToTimeOption(
  startDate: Date,
  requested = false
): TimeOption {
  if (!startDate) {
    return null;
  }

  const time = {
    label: dateTimeShortFormat(startDate),
    value: startDate,
  };

  if (requested) {
    time.label += ' (Requested)';
  }

  return time;
}

/**
 * Calculates selectable time options based on the day user has selected.
 * There are limits to the earliest time option selectable and to the
 * latest time option selectable based on the time options (`created` and
 * `maxDuration`) returned from the initial dry run access request.
 */
export function getTimeOptions(
  selectedDate: Date,
  accessRequest: AccessRequest,
  reviewing = false
): TimeOption[] {
  const maxAssumableDate = getMaxAssumableDate(accessRequest);
  let minDate = accessRequest.created;
  if (reviewing) {
    // Give reviewer the time options starting from now
    // otherwise we can render time options in the past
    // (reviewing a request after some time has passed).
    minDate = new Date();
  }

  const maxAssumableDuration = maxAssumableDate.getTime();

  // Today was the only day available to select.
  // This means there will be both a start limit (earliest time selectable)
  // and a end limit (latest time selectable).
  if (
    isSameDay(selectedDate, minDate) &&
    isSameDay(selectedDate, maxAssumableDate)
  ) {
    return generateTimeDropdown({
      minTimestamp: minDate.getTime(),
      maxTimestamp: maxAssumableDuration,
      startDate: selectedDate,
    });
  }
  // User selected the first day among other selectable days.
  // This means there is only a start limit (earliest time selectable)
  // and end is only limited to the last time available for the day (23:59)
  else if (isSameDay(selectedDate, minDate)) {
    return generateTimeDropdown({
      minTimestamp: minDate.getTime(),
      startDate: selectedDate,
    });
  }
  // User selected the last day among other selectable days.
  // This means there is only a end limit (latest time selectable) and
  // start is only limited to the earliest time available for the day (00:00)
  else if (isSameDay(selectedDate, maxAssumableDate)) {
    return generateTimeDropdown({
      maxTimestamp: maxAssumableDuration,
      startDate: selectedDate,
    });
  }
  // User selected in between the first and last date selectable, so
  // any time options are selectable (00:00 - 23:59)
  else {
    return generateTimeDropdown({ startDate: selectedDate });
  }
}

/**
 * Selects the lesser value between maxDuration
 * and default max assume start time (one week).
 *
 * Note: maxDuration backend limit can be up to 2 weeks.
 */
export function getMaxAssumableDate({
  created,
  maxDuration,
}: {
  created: Date;
  maxDuration: Date;
}) {
  let maxAssumableDate = addDays(created, OneWeek);

  // Select the lesser value.
  if (maxAssumableDate.getTime() > maxDuration.getTime()) {
    maxAssumableDate = new Date(maxDuration);
  }

  // Subtract an hour off the max duration so we don't display
  // options that is too near the max duration (worst case the
  // request has like a few seconds before it expires)
  const modifiedMaxAssumableDate = new Date(maxAssumableDate);
  modifiedMaxAssumableDate.setHours(maxAssumableDate.getHours() - 1);

  // Handles an edge case where the max duration is less than an hour.
  if (modifiedMaxAssumableDate.getTime() < created.getTime()) {
    return maxAssumableDate;
  }

  return modifiedMaxAssumableDate;
}
