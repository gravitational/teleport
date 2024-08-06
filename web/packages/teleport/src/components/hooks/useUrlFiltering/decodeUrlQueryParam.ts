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

/**
 * calls native decodeURIComponent under the hood but goes
 * one step further by replacing lone % with %25
 * to prevent URI malformed error.
 */
export function decodeUrlQueryParam(param: string) {
  const decodedQuery = decodeURIComponent(
    param.replace(/%(?![0-9][0-9a-fA-F]+)/g, '%25')
  );

  return decodedQuery;
}
