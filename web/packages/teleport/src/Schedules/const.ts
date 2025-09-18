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
 * timezoneOptions lists the available timezone options.
 */
export const timezoneOptions: Option[] = allTimezones.map(timeZone => {
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
 * weekdayOptions lists the available weekday options.
 */
export const weekdayOptions: WeekdayOption[] = [
  { value: 'Sunday', label: 'S' },
  { value: 'Monday', label: 'M' },
  { value: 'Tuesday', label: 'T' },
  { value: 'Wednesday', label: 'W' },
  { value: 'Thursday', label: 'T' },
  { value: 'Friday', label: 'F' },
  { value: 'Saturday', label: 'S' },
];

/**
 * timeOptionsAll lists the available time options per minute.
 * The value is 24 hour formatted, while the label is 12 hour formatted.
 */
export const timeOptionsAll: Option[] = Array.from(
  { length: 1440 },
  (_, index) => {
    const hours = Math.floor(index / 60);
    const minutes = index % 60;

    const date = new Date();
    date.setHours(hours, minutes, 0, 0);

    const parts = new Intl.DateTimeFormat('en-US', {
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    }).formatToParts(date);

    const label =
      parts.find(p => p.type === 'hour')?.value +
      ':' +
      parts.find(p => p.type === 'minute')?.value +
      parts.find(p => p.type === 'dayPeriod')?.value;

    const value = `${hours.toString().padStart(2, '0')}:${minutes
      .toString()
      .padStart(2, '0')}`;

    return { value, label };
  }
);

/**
 * timeOptions lists the available time options with 30-minute intervals.
 * The value is 24 hour formatted, while the label is 12 hour formatted.
 */
export const timeOptions: Option[] = timeOptionsAll.filter(option => {
  const minutes = option.value.split(':')[1];
  return minutes === '00' || minutes === '30';
});
