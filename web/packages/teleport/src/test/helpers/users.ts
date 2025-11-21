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

import { User } from 'teleport/services/user';

interface BadRequest {
  error: {
    message: string;
  };
}

interface UsersV2Response {
  items: User[];
  startKey?: string;
}

const usersPath = '/v1/webapi/users';
const usersPathV2 = '/v2/webapi/users';

export function handleGetUsers(
  resolver: HttpResponseResolver<never, never, User[] | BadRequest>
) {
  return http.get(usersPath, resolver);
}

export function handleGetUsersV2(
  resolver: HttpResponseResolver<never, never, UsersV2Response | BadRequest>
) {
  return http.get(usersPathV2, resolver);
}

export function handleUpdateUser(
  resolver: HttpResponseResolver<never, never, User | BadRequest>
) {
  return http.put(usersPath, resolver);
}

export function handleDeleteUser(
  resolver: HttpResponseResolver<never, never, { message: string } | BadRequest>
) {
  return http.delete(usersPath + '/*', resolver);
}

export const successGetUsers = (users: User[]) =>
  handleGetUsers(() => HttpResponse.json(users));

export const successGetUsersV2 = (users: User[], startKey?: string) =>
  handleGetUsersV2(() => HttpResponse.json({ items: users, startKey }));

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
export const successUpdateUser = () =>
  handleUpdateUser(async res => HttpResponse.json(await res.request.json()));

export const successDeleteUser = () =>
  handleDeleteUser(() => HttpResponse.json({ message: 'ok' }));

export const errorDeleteUser = (message: string) =>
  handleDeleteUser(() =>
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
