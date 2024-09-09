/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

export function makeAdvancedSearchQueryForLabel(
  label: {
    name: string;
    value: string;
  },
  params: {
    /** Contains search words/phrases separated by space. */
    search?: string;
    /** Query expression using the predicate language. */
    query?: string;
  }
): string {
  const queryParts: string[] = [];

  // Add an existing query.
  if (params.query) {
    queryParts.push(params.query);
  }

  // If there is an existing simple search, convert it to predicate language and add it.
  if (params.search) {
    queryParts.push(`search("${params.search}")`);
  }

  const labelQuery = `labels["${label.name}"] == "${label.value}"`;
  queryParts.push(labelQuery);

  return queryParts.join(' && ');
}
