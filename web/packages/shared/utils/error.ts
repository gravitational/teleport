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

/**
 * Converts any unknown input into an Error instance.
 * Preserves message and name if available.
 */
export function ensureError(err: unknown): Error {
  if (err instanceof Error) {
    return err;
  }

  if (err === null || err === undefined) {
    return new Error();
  }

  if (typeof err === 'object') {
    let message: string;
    if ('message' in err) {
      message = String(err.message);
    } else {
      message = JSON.stringify(err);
    }

    const error = new Error(message);
    if ('name' in err && typeof err.name === 'string') {
      error.name = err.name;
    }
    return error;
  }

  return new Error(String(err));
}

/** Extracts an error message or returns a default one. */
export function getErrorMessage(err: unknown): string {
  const errorInstance = ensureError(err);
  if (errorInstance.message === '') {
    return 'something went wrong';
  }
  return errorInstance.message;
}
