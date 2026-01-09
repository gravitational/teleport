/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { FilterMap } from './ListFilters';

export function applyFilters<Item, Values extends Record<string, unknown>>(
  list: Item[],
  filters: FilterMap<Item, Values>
) {
  return Object.values(filters).reduce((acc, filter) => {
    if (filter.selected.length === 0) return acc;
    return filter.apply(acc, filter.selected);
  }, list);
}
