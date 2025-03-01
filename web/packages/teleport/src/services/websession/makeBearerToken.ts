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

import { BearerToken } from './types';

export default function makeBearerToken(json: any): BearerToken {
  return {
    accessToken: json.token,
    expiresIn: json.expires_in,
    created: new Date().getTime(),
    sessionExpires: json.sessionExpires,
    sessionExpiresIn: json.sessionExpiresIn,
    sessionInactiveTimeout: json.sessionInactiveTimeout,
  };
}
