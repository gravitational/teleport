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

import { RpcError } from '@protobuf-ts/runtime-rpc';

import { deserializeError, serializeError } from './ipcSerializer';

test('serializes and deserializes regular error', () => {
  const err = new Error('Boom');
  err.name = 'CustomError';

  const serialized = serializeError(err);
  expect(serialized instanceof Error).toBe(false);
  expect(serialized.message).toBe('Boom');
  expect(serialized.name).toBe('CustomError');
  expect(serialized.stack).toBe(err.stack);
  expect(serialized.cause).toBe(err.cause);

  const cloned = structuredClone(serialized);
  const deserialized = deserializeError(cloned);
  expect(deserialized instanceof Error).toBe(true);
  expect(deserialized.message).toBe('Boom');
  expect(deserialized.name).toBe('CustomError');
  expect(deserialized.stack).toBe(err.stack);
  expect(deserialized.cause).toBe(err.cause);
});

test('serializes and deserializes RPC error', () => {
  const err = new RpcError('Could not found', 'NOT_FOUND', {
    'is-resolvable-with-relogin': ['1'],
  });

  const serialized = serializeError(err);
  expect(serialized instanceof Error).toBe(false);
  expect(serialized.message).toBe('Could not found');
  expect(serialized.name).toBe('RpcError');
  expect(serialized['code']).toBe('NOT_FOUND');
  expect(serialized['meta']).toEqual({
    'is-resolvable-with-relogin': ['1'],
  });

  const cloned = structuredClone(serialized);
  const deserialized = deserializeError(cloned);
  expect(deserialized instanceof Error).toBe(true);
  expect(deserialized.message).toBe('Could not found');
  expect(deserialized.name).toBe('RpcError');
  expect(deserialized['code']).toBe('NOT_FOUND');
  expect(deserialized['meta']).toEqual({
    'is-resolvable-with-relogin': ['1'],
  });
});
