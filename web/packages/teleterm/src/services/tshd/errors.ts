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

import { isTshdRpcError } from './cloneableClient';

export function isAccessDeniedError(error: unknown): boolean {
  // TODO(gzdunek): Replace it with check on the code field.
  if (isTshdRpcError(error)) {
    return error.message.includes('access denied');
  }
  return false;
}

export function isNotFoundError(error: unknown): boolean {
  if (isTshdRpcError(error)) {
    return error.code === 'NOT_FOUND';
  }
  return false;
}

export function isUnimplementedError(error: unknown): boolean {
  if (isTshdRpcError(error)) {
    return error.code === 'UNIMPLEMENTED';
  }
  return false;
}
