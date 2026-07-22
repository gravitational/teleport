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

import { ensureError, isAbortError } from './error';

class CustomErrorClass extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'CustomErrorClass';
  }
}

describe('ensureError', () => {
  const cases = [
    {
      input: new Error('already error'),
      expectedMessage: 'already error',
      expectedName: 'Error',
      expectedInstance: Error,
    },
    {
      input: { message: 'custom message' },
      expectedMessage: 'custom message',
      expectedName: 'Error',
      expectedInstance: Error,
    },
    {
      input: { message: '', otherField: '123' },
      expectedMessage: '',
      expectedName: 'Error',
      expectedInstance: Error,
    },
    {
      input: { name: 'MyError', message: 'fail' },
      expectedMessage: 'fail',
      expectedName: 'MyError',
      expectedInstance: Error,
    },
    {
      input: new CustomErrorClass('fail'),
      expectedMessage: 'fail',
      expectedName: 'CustomErrorClass',
      expectedInstance: CustomErrorClass,
    },
    {
      input: { foo: 'bar' },
      expectedMessage: '{"foo":"bar"}',
      expectedName: 'Error',
      expectedInstance: Error,
    },
    {
      input: 'just a string',
      expectedMessage: 'just a string',
      expectedName: 'Error',
      expectedInstance: Error,
    },
    {
      input: 42,
      expectedMessage: '42',
      expectedName: 'Error',
      expectedInstance: Error,
    },
    {
      input: null,
      expectedMessage: '',
      expectedName: 'Error',
      expectedInstance: Error,
    },
    {
      input: undefined,
      expectedMessage: '',
      expectedName: 'Error',
      expectedInstance: Error,
    },
  ];

  test.each(cases)(
    'converts input "$input" to Error with message "$expectedMessage" and name "$expectedName"',
    ({ input, expectedMessage, expectedName, expectedInstance }) => {
      const error = ensureError(input);

      expect(error).toBeInstanceOf(expectedInstance);
      expect(error.message).toBe(expectedMessage);
      expect(error.name).toBe(expectedName);
      // Non-Error instances should have the original input attached as .cause.
      expect(error.cause).toBe(input instanceof Error ? undefined : input);
    }
  );
});

describe('isAbortError', () => {
  describe.each([
    ['DOMException', newDOMAbortError],
    ['ApiError', newApiAbortError],
    ['gRPC Error', newGrpcAbortError],
  ])('for error type %s', (_, ErrorType) => {
    it('is abort error', () => {
      expect(isAbortError(ErrorType())).toBe(true);
    });
  });
});

function newDOMAbortError() {
  return new DOMException('Aborted', 'AbortError');
}

// mimics ApiError
function newApiAbortError() {
  return new Error('The user aborted a request', {
    cause: newDOMAbortError(),
  });
}

// mimics TshdRpcError
function newGrpcAbortError() {
  return { code: 'CANCELLED' };
}
