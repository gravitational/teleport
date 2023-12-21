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

export const isAbortError = (err: any): boolean => {
  // handles Web UI abort error
  if (
    (err instanceof DOMException && err.name === 'AbortError') ||
    (err.cause && isAbortError(err.cause))
  ) {
    return true;
  }

  // handles Connect abort error (specifically gRPC cancel error)
  // the error has only the message field that contains the following string:
  // '1 CANCELLED: Cancelled on client'
  return err instanceof Error && err.message?.includes('CANCELLED');
};
