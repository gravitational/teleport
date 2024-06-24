/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { AccessRequest, Resource } from 'shared/services/accessRequests';

// TODO
// places that use this need to be replaced with server-side filtering like
// done in RequestList.tsx:
// https://github.com/gravitational/teleport.e/blob/a776b3e65e6e8e11ca6938025e63fc3676238fb2/web/teleport/src/Workflow/ReviewRequests/RequestList/RequestList.tsx#L72
export function requestMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof AccessRequest
) {
  if (propName === 'roles') {
    return targetValue.some((role: string) =>
      role.toUpperCase().includes(searchValue)
    );
  }

  if (propName === 'resources') {
    return targetValue.some((r: Resource) =>
      Object.values(r.id)
        .concat(Object.values(r.details.hostname || {}))
        .concat(Object.values(r.details.friendlyName || {}))
        .some(v => v.toUpperCase().includes(searchValue))
    );
  }
}
