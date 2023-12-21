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

export default function parseError(json) {
  let msg = '';

  if (json && json.error) {
    msg = json.error.message;
  } else if (json && json.message) {
    msg = json.message;
  } else if (json.responseText) {
    msg = json.responseText;
  }
  return msg;
}

export class ApiError extends Error {
  response: Response;

  constructor(message: string, response: Response, opts?: ErrorOptions) {
    message = message || 'Unknown error';
    super(message, opts);
    this.response = response;
    this.name = 'ApiError';
  }
}
