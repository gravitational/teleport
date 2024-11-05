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

/** Breaks the `data` array up into chunks the length of `pageSize`. */
export default function paginateData<T>(data: T[] = [], pageSize = 10): T[][] {
  const pageCount = Math.ceil(data.length / pageSize);
  const pages = [];

  for (let i = 0; i < pageCount; i++) {
    const start = i * pageSize;
    const page = data.slice(start, start + pageSize);
    pages.push(page);
  }

  // If there are no items, place an empty page inside pages
  if (pages.length === 0) {
    pages[0] = [];
  }

  return pages;
}
