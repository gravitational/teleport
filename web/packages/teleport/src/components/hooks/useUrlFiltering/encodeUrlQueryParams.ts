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

import { SortType } from 'design/DataTable/types';
import { ResourceHealthStatus } from 'shared/components/UnifiedResources';

export type EncodeUrlQueryParamsProps = {
  pathname: string;
  searchString?: string;
  sort?: SortType | null;
  kinds?: string[] | null;
  statuses?: ResourceHealthStatus[] | null;
  isAdvancedSearch?: boolean;
  pinnedOnly?: boolean;
};

export function encodeUrlQueryParams({
  pathname,
  searchString = '',
  sort,
  kinds,
  isAdvancedSearch = false,
  pinnedOnly = false,
  statuses,
}: EncodeUrlQueryParamsProps) {
  const urlParams = new URLSearchParams();

  if (searchString) {
    urlParams.append(isAdvancedSearch ? 'query' : 'search', searchString);
  }

  if (sort) {
    urlParams.append('sort', `${sort.fieldName}:${sort.dir.toLowerCase()}`);
  }

  if (pinnedOnly !== undefined) {
    urlParams.append('pinnedOnly', `${pinnedOnly}`);
  }

  if (kinds) {
    for (const kind of kinds) {
      urlParams.append('kinds', kind);
    }
  }

  if (statuses) {
    for (const status of statuses) {
      urlParams.append('status', status);
    }
  }

  const encodedParams = urlParams.toString();

  return encodedParams ? `${pathname}?${encodedParams}` : pathname;
}
