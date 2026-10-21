/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import cfg from 'teleport/config';

import api from '../api';

export type ListAssignedScopesResponse = {
  scopes: string[];
  nextPageToken?: string;
};

export async function listAssignedScopes(
  { prefix = '', pageToken = '' }: { prefix?: string; pageToken?: string },
  signal: AbortSignal
): Promise<ListAssignedScopesResponse> {
  const params = new URLSearchParams();
  if (prefix) {
    params.set('prefix', prefix);
  }
  if (pageToken) {
    params.set('page_token', pageToken);
  }

  const res = await api.get(
    `${cfg.api.listAssignedScopes}?${params.toString()}`,
    signal
  );
  return {
    scopes: res.scopes ?? [],
    nextPageToken: res.next_page_token,
  };
}
