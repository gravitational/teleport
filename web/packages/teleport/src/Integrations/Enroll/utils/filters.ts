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

import { type IntegrationTag } from 'teleport/Integrations/Enroll';

export type FilterableIntegration<T> = T &
  ({ name: string; title?: never } | { title: string; name?: never }) & {
    tags: IntegrationTag[];
  };

export const titleOrName = <T>(i: FilterableIntegration<T>) => {
  if ('title' in i) {
    return i.title;
  } else if ('name' in i) {
    return i.name;
  }
};

export function filterIntegrations<T>(
  integrations: FilterableIntegration<T>[],
  tags: IntegrationTag[],
  search: string
): FilterableIntegration<T>[] {
  if (!integrations.length && !tags && search === '') {
    return integrations;
  }

  const searches = search.split(' ').map(s => s.toLowerCase());

  const found = integrations.filter(i =>
    searches.every(s => titleOrName(i).toLowerCase().includes(s))
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
