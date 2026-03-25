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

import { IntegrationLike } from './IntegrationList';
import { getStatus } from './shared/StatusLabel';
import { Status } from './types';

const StatusRank: Record<Status, number> = {
  [Status.Draft]: 1,
  [Status.Scanning]: 2,
  [Status.Healthy]: 3,
  [Status.Issues]: 4,
  [Status.Failed]: 5,
  [Status.Unknown]: 6,
};

export const sortByStatus = (a, b) => {
  const { status: statusA } = getStatus(a);
  const { status: statusB } = getStatus(b);
  return StatusRank[statusA] - StatusRank[statusB];
};

export function filterByIntegrationStatus(
  l: IntegrationLike[],
  s: Status[]
): IntegrationLike[] {
  return l.filter(i => {
    if (s.length) {
      const { status } = getStatus(i);
      if (!s.includes(status)) {
        return false;
      }
    }
    return true;
  });
}

export function filterBySearch(
  l: IntegrationLike[],
  s: string
): IntegrationLike[] {
  const search = s.trim().toLocaleUpperCase();
  if (!search) return l;

  return l.filter(i => {
    return (
      i.name.toLocaleUpperCase().includes(search) ||
      i.kind.toLocaleUpperCase().includes(search) ||
      (i.details && i.details.toLocaleUpperCase().includes(search))
    );
  });
}
