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

import { Rule } from 'shared/components/Validation/rules';

import { timeOptionsAll } from './const';
import { Schedule, Shift } from './types';

/**
 * validSchedule validates a schedule.
 * A schedule is expected to contain at least one shift, and all shifts
 * must be valid.
 *
 * @param schedule - The target schedule to validate.
 * @returns a validator function that ensure the schedule is valid.
 */
export const validSchedule: Rule<Schedule> = (schedule: Schedule) => () => {
  const enabledShifts = Object.values(schedule.shifts).filter(shift => !!shift);

  if (enabledShifts.length === 0) {
    return {
      valid: false,
      message: `At least one shift is required.`,
    };
  }

  for (const shift of enabledShifts) {
    const error = validateShift(shift);
    if (error) {
      return {
        valid: false,
        message: `Shift must be between 12:00AM and 11:59PM.`,
      };
    }
  }

  return {
    valid: true,
  };
};

/**
 * validShift validates a shift.
 * A shift is expected to be between 00:00 and 23:59.
 *
 * @param shift - The target shift to validate.
 * @returns a validator function that ensure the shift is valid.
 */
export const validShift: Rule<Shift> = (shift: Shift) => () => {
  const error = validateShift(shift);
  return {
    valid: !error,
    message: error,
  };
};

const validateShift = (shift: Shift) => {
  const start = shift.startTime?.value;
  const end = shift.endTime?.value;

  if (!start || !end) {
    return 'start and end interval required';
  }

  const startIndex = timeOptionsAll.findIndex(t => t.value === start);
  const endIndex = timeOptionsAll.findIndex(t => t.value === end);

  if (startIndex === -1 || endIndex === -1) {
    return 'invalid time';
  }

  if (startIndex >= endIndex) {
    return 'start time must be before end time';
  }
  return undefined;
};
