/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { displayDate, displayDateTime } from 'shared/services/loc';

import { MatchCallback } from './match';

export function dateMatcher<T>(
  datePropNames: (keyof T & string)[]
): MatchCallback<T> {
  return (targetValue, searchValue, propName) => {
    if (datePropNames.includes(propName)) {
      return displayDate(new Date(targetValue))
        .toLocaleUpperCase()
        .includes(searchValue);
    }
  };
}

export function dateTimeMatcher<T>(
  dateTimePropNames: (keyof T & string)[]
): MatchCallback<T> {
  return (targetValue, searchValue, propName) => {
    if (dateTimePropNames.includes(propName)) {
      return displayDateTime(new Date(targetValue))
        .toLocaleUpperCase()
        .includes(searchValue);
    }
  };
}
