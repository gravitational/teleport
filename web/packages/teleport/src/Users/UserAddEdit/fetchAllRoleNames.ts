/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

type RolePage = {
  items: { name: string }[];
  startKey: string;
};

const ROLES_PAGE_SIZE = 50;

// Stops a runaway loop if the server keeps returning a next key.
const MAX_ROLE_PAGES = 40;

/**
 * Fetches the names of every role matching `search`, following the next-page
 * key until the server has no more pages.
 */
export async function fetchAllRoleNames(
  fetchPage: (params: {
    search: string;
    limit: number;
    startKey: string;
  }) => Promise<RolePage>,
  search: string
): Promise<string[]> {
  const names: string[] = [];
  let startKey = '';
  for (let page = 0; page < MAX_ROLE_PAGES; page++) {
    const response = await fetchPage({
      search,
      limit: ROLES_PAGE_SIZE,
      startKey,
    });
    for (const role of response.items) {
      names.push(role.name);
    }
    startKey = response.startKey;
    if (!startKey) {
      break;
    }
  }
  return names;
}
