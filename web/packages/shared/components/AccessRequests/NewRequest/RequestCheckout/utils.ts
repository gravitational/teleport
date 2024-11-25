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

import { addDays, addHours, isAfter } from 'date-fns';

import { Option } from 'shared/components/Select';

import { getFormattedDurationTxt } from '../../Shared/utils';

// Preset hour options for the time option dropdowns.
export const presetHours = [1, 2, 3, 4, 6, 8, 12, 18];
// Preset day options for the time option dropdowns.
export const presetDays = [1, 2, 3, 4, 5, 6, 7];
const HourInMs = 60 * 60 * 1000;

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
  const possiblySmallerMaxPendingDate = addDays(
    createdDate,
    presetDays[maxPendingDay]
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
      const updatedEndDate = addDays(createdDate, day);
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
