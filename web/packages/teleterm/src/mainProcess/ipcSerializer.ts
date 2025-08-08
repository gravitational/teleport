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

export type SerializedError = {
  name: string;
  message: string;
  stack?: string;
  cause?: unknown;
};

/** Serializes an Error into a plain object for transport through Electron IPC. */
export function serializeError(error: Error): SerializedError {
  return {
    name: error.name,
    message: error.message,
    cause: error.cause,
    stack: error.stack,
  };
}

/** Deserializes a plain object back into an Error instance. */
export function deserializeError(serialized: SerializedError): Error {
  const error = new Error(serialized.message);
  error.name = serialized.name;
  error.cause = serialized.cause;
  error.stack = serialized.stack;
  return error;
}
