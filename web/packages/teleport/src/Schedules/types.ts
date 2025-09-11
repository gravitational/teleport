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

import { Option } from 'shared/components/Select';

import { TimeOptions, TimezoneOptions } from './const';

export type Schedule = {
  name: string;
  timezone: Option;
  shifts: Partial<Record<Weekday, Shift>>;
};

export const NewSchedule = (): Schedule => ({
  name: 'default',
  timezone: TimezoneOptions[0],
  shifts: {},
});

export type Shift = {
  startTime: Option;
  endTime: Option;
};

export const NewShift = (): Shift => ({
  startTime: TimeOptions[0],
  endTime: TimeOptions[0],
});

export type Weekday =
  | 'Sunday'
  | 'Monday'
  | 'Tuesday'
  | 'Wednesday'
  | 'Thursday'
  | 'Friday'
  | 'Saturday';

export type WeekdayOption = Option<Weekday>;
