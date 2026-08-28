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

import { ensureError } from 'shared/utils/error';

export type SerializedError = {
  name: string;
  message: string;
  stack?: string;
  cause?: unknown;
  toStringResult?: string;
  $serializedError?: true;
};

function isSerialized(error: unknown): error is SerializedError {
  return (
    !!error &&
    typeof error === 'object' &&
    error['$serializedError'] === true &&
    typeof error['name'] === 'string' &&
    typeof error['message'] === 'string'
  );
}

/** Serializes an error into a plain object for transport through Electron IPC. */
export function serializeError(error: unknown): SerializedError {
  if (isSerialized(error)) {
    return error;
  }

  const errorInstance = ensureError(error);
  const {
    name,
    cause,
    stack,
    message,
    // functions must be skipped, otherwise structuredClone will fail to clone the object
    toString,
    ...enumerableFields
  } = errorInstance;
  return {
    name,
    message,
    cause,
    stack,
    // Calling the destructured function directly could result in the following error:
    // Method Error.prototype.toString called on incompatible receiver undefined
    toStringResult: errorInstance.toString?.(),
    ...enumerableFields,
    $serializedError: true,
  };
}

/** Deserializes a plain object back into an Error instance. */
export function deserializeError(serialized: SerializedError): Error {
  const {
    name,
    cause,
    stack,
    message,
    toStringResult,
    $serializedError,
    ...rest
  } = serialized;
  const error = new Error(message);
  error.name = name;
  error.cause = cause;
  error.stack = stack;
  error.toString = () => toStringResult;
  Object.assign(error, rest);
  return error;
}
