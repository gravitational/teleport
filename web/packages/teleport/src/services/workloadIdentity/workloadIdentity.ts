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

import cfg from 'teleport/config';

import api from '../api/api';
import { withGenericUnsupportedError } from '../version/unsupported';
import { validateListWorkloadIdentitiesResponse } from './consts';

export async function listWorkloadIdentities(
  variables: {
    pageToken: string;
    pageSize: number;
    sortField: string;
    sortDir: string;
    searchTerm?: string;
  },
  signal?: AbortSignal
) {
  const { pageToken, pageSize, sortField, sortDir, searchTerm } = variables;

  const path = cfg.getWorkloadIdentityUrl({ action: 'list' });
  const qs = new URLSearchParams();

  qs.set('page_size', pageSize.toFixed());
  qs.set('page_token', pageToken);
  if (sortField) {
    qs.set('sort_field', sortField);
  }
  if (sortDir) {
    qs.set('sort_dir', sortDir);
  }
  if (searchTerm) {
    qs.set('search', searchTerm);
  }

  try {
    const data = await api.get(`${path}?${qs.toString()}`, signal);

    if (!validateListWorkloadIdentitiesResponse(data)) {
      throw new Error('failed to validate list workload identities response');
    }

    return data;
  } catch (err) {
    // TODO(nicholasmarais1158) DELETE IN v20.0.0
    withGenericUnsupportedError(err, '19.0.0');
  }
}
