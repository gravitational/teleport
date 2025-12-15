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

import { http, HttpResponse, HttpResponseResolver } from 'msw';

import { RoleResource } from 'teleport/services/resources';

const rolesPath = '/v1/webapi/roles';

interface GetRolesResponse {
  items: RoleResource[];
  startKey: string;
}

function handleGetRoles(resolver: HttpResponseResolver) {
  return http.get(rolesPath, resolver);
}

export const successGetRoles = (res: GetRolesResponse) =>
  handleGetRoles(() => HttpResponse.json(res));

export const errorGetRoles = (message: string) =>
  handleGetRoles(() =>
    HttpResponse.json(
      {
        error: {
          message,
        },
      },
      {
        status: 400,
      }
    )
  );
