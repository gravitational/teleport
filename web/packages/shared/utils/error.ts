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
export function ensureError(input: unknown): Error {
  if (input instanceof Error) {
    return input;
  }

  if (input === null || input === undefined) {
    return new Error('', { cause: input });
  }

  if (typeof input !== 'object') {
    return new Error(String(input), { cause: input });
  }

  let message = '';
  if ('message' in input) {
    message = String(input.message);
  } else {
    try {
      message = JSON.stringify(input);
    } catch {
      message = '[Unable to stringify the thrown value]';
    }
  }

  const error = new Error(message, { cause: input });
  if ('name' in input && typeof input.name === 'string') {
    error.name = input.name;
  }
  return error;
}

/** Extracts an error message or returns a default one. */
export function getErrorMessage(err: unknown): string {
  const errorInstance = ensureError(err);
  if (errorInstance.message === '') {
    return 'something went wrong';
  }
  return errorInstance.message;
}

export function isAbortError(err: any): boolean {
  // handles Web UI abort error
  if (
    (err instanceof DOMException && err.name === 'AbortError') ||
    (err?.cause && isAbortError(err.cause))
  ) {
    return true;
  }

  // handles Connect abort error (specifically gRPC cancel error), see TshdRpcError
  return err?.code === 'CANCELLED';
}
