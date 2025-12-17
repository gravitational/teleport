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

import { TshdRpcError } from 'teleterm/services/tshd/cloneableClient';

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

test('serializes and deserializes tshd error', () => {
  const err: TshdRpcError = {
    name: 'TshdRpcError',
    message: 'Could not found',
    toString: () => 'Could not found',
    cause: '',
    stack: '',
    code: 'NOT_FOUND',
    isResolvableWithRelogin: false,
  };

  const serialized = serializeError(err);
  expect(serialized instanceof Error).toBe(false);
  expect(serialized.message).toBe('Could not found');
  expect(serialized.name).toBe('TshdRpcError');
  expect(serialized['isResolvableWithRelogin']).toBe(false);
  expect(serialized['code']).toBe('NOT_FOUND');

  const cloned = structuredClone(serialized);
  const deserialized = deserializeError(cloned);
  expect(deserialized instanceof Error).toBe(true);
  expect(deserialized.message).toBe('Could not found');
  expect(deserialized.name).toBe('TshdRpcError');
  expect(deserialized['isResolvableWithRelogin']).toBe(false);
  expect(deserialized['code']).toBe('NOT_FOUND');
});
