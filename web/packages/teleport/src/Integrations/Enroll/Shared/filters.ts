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

import {
  displayName,
  type BaseIntegration,
  type IntegrationTag,
} from 'teleport/Integrations/Enroll/Shared';

export function filterIntegrations<T extends BaseIntegration>(
  integrations: T[],
  tags: IntegrationTag[],
  search: string
): T[] {
  if (!integrations.length && !tags && search === '') {
    return integrations;
  }

  const searches = search.split(' ').map(s => s.toLowerCase());

  const found = integrations.filter(i =>
    searches.every(
      s =>
        displayName<T>(i).toLowerCase().includes(s) ||
        i.tags.some(tag => tag.includes(s)) ||
        i.description?.toLowerCase().includes(s)
    )
  );

  let filtered = [...found];

  if (tags.length) {
    filtered = filtered.filter(i => {
      if ('tags' in i) {
        return tags.some(tag => i.tags.includes(tag));
      }
    });
  }

  return filtered;
}
