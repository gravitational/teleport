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

import { pluralize } from './text';

export function getDurationText(hrs: number, mins: number, secs: number) {
  if (!hrs && !mins) {
    return `${secs} secs`;
  }

  const hrText = pluralize(hrs, 'hr');
  const minText = pluralize(mins, 'min');

  if (!hrs) {
    return `${mins} ${minText}`;
  }

  if (hrs && !mins) {
    return `${hrs} ${hrText}`;
  }

  return `${hrs} ${hrText} and ${mins} ${minText}`;
}
