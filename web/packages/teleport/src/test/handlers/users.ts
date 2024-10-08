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

import { http, HttpResponse, HttpResponseResolver } from 'msw';

import { User } from 'teleport/services/user';

interface BadRequest {
  error: {
    message: string;
  };
}

export function handleGetUsers(
  resolver: HttpResponseResolver<never, never, User[] | BadRequest>
) {
  return http.get('/v1/webapi/users', resolver);
}

export const successGetUsers = (users: User[]) =>
  handleGetUsers(() => HttpResponse.json(users));

export const errorGetUsers = (message: string) =>
  handleGetUsers(() =>
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
