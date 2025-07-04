/**
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

import { SortType } from './types';

/**
 * parseSortType converts the "fieldname:dir" format into {fieldName: "", dir: ""}
 * @param sort the sort string to parse
 * @returns the parsed sort type or null if the sort string is invalid
 */
export function parseSortType(sort: string): SortType | null {
  if (!sort) return null;
  const [fieldName, dir] = sort.split(':');
  if (!fieldName) return null;
  return {
    fieldName,
    dir: dir?.toLowerCase() === 'desc' ? 'DESC' : 'ASC',
  };
}

/**
 * formatSortType converts a SortType ({fieldName: "", dir: ""}) to the "fieldname:dir" format
 * @param sortType the sort to format
 * @returns the formatted sort string
 */
export function formatSortType(sortType: SortType): string {
  return `${sortType.fieldName}:${sortType.dir.toLowerCase()}`;
}
