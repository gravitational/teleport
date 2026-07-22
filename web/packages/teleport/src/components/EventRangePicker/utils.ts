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

import { endOfDay, startOfDay, subDays } from 'date-fns';

export function getRangeOptions(): EventRange[] {
  return [
    {
      name: 'Today',
      from: startOfDay(new Date()),
      to: endOfDay(new Date()),
    },
    {
      name: '7 days',
      from: startOfDay(subDays(new Date(), 6)),
      to: endOfDay(new Date()),
    },
    {
      name: 'Custom Range...',
      isCustom: true,
      from: new Date(),
      to: new Date(),
    },
  ];
}

export type EventRange = {
  from: Date;
  to: Date;
  isCustom?: boolean;
  name?: string;
};
