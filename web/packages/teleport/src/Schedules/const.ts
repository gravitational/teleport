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

import { WeekdayOption } from './types';

const allTimezones = ['UTC', ...Intl.supportedValuesOf('timeZone')];

/**
 * TimezoneOptions lists the available timezone options.
 */
export const TimezoneOptions: Option[] = allTimezones.map(timeZone => {
  const formatter = new Intl.DateTimeFormat('en-US', {
    timeZone,
    timeZoneName: 'short',
  });

  const abbreviation =
    formatter
      .formatToParts(new Date())
      .find(part => part.type === 'timeZoneName')?.value || '';

  return {
    value: timeZone,
    label: `${timeZone} - ${abbreviation}`,
  };
});

/**
 * WeekdayOptions lists the available weekday options.
 */
export const WeekdayOptions: WeekdayOption[] = [
  { value: 'Sunday', label: 'S' },
  { value: 'Monday', label: 'M' },
  { value: 'Tuesday', label: 'T' },
  { value: 'Wednesday', label: 'W' },
  { value: 'Thursday', label: 'T' },
  { value: 'Friday', label: 'F' },
  { value: 'Saturday', label: 'S' },
];

/**
 * TimeOptions lists the available time options with 30-minute intervals.
 */
export const TimeOptions: Option[] = Array.from({ length: 48 }, (_, index) => {
  const hours = Math.floor(index / 2);
  const minutes = index % 2 === 0 ? '00' : '30';
  const time = `${hours.toString().padStart(2, '0')}:${minutes}`;
  return { value: time, label: time };
});
