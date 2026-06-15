/**
 * @jest-environment node
 */
/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { EventEmitter } from 'node:events';

import {
  ClientStreamingCall,
  DuplexStreamingCall,
  MethodInfo,
  RpcError,
  RpcOutputStream,
  ServerStreamingCall,
  ServiceInfo,
  UnaryCall,
} from '@protobuf-ts/runtime-rpc';

import {
  cloneAbortSignal,
  cloneClient,
  isRpcError,
  isRpcErrorReloginResolvable,
} from './cloneableClient';

function getRpcError() {
  return new RpcError('You do not have permission.', 'ACCESS_DENIED', {
    'is-resolvable-with-relogin': ['1'],
  });
}

const rpcErrorObjectMatcher = expect.objectContaining<RpcError>({
  code: 'ACCESS_DENIED',
  message: 'You do not have permission.',
  name: 'RpcError',
  meta: {
    'is-resolvable-with-relogin': ['1'],
  },
  stack: expect.stringContaining('You do not have permission.'),
  cause: undefined,
});

class MockServiceMethod<
  T extends (...args: any[]) => any,
> implements ServiceInfo {
  public methods: MethodInfo[];
  public options = {};
  public typeName = '';

  constructor(
    methodInfo: Pick<MethodInfo, 'clientStreaming' | 'serverStreaming'>,
    private implementation: T
  ) {
    this.methods = [
      {
        localName: 'fakeMethod',
        serverStreaming: methodInfo.serverStreaming,
        clientStreaming: methodInfo.clientStreaming,
      } as MethodInfo,
    ];
  }

  fakeMethod(...args: Parameters<T>): ReturnType<T> {
    return this.implementation(...args);
  }
}

test('cloneable abort signal reads up-to-date signal.aborted and signal.reason', () => {
  const controller = new AbortController();
  const cloned = cloneAbortSignal(controller.signal);
  expect(cloned.aborted).toBe(false);
  expect(cloned.reason).toBeUndefined();

  controller.abort('test reason');
  expect(cloned.aborted).toBe(true);
  expect(cloned.reason).toBe('test reason');
});

test('response error is cloned as an object for a unary call', async () => {
  const fakeCall: () => UnaryCall = jest.fn().mockImplementation(() => ({
    then: () => Promise.reject(getRpcError()),
  }));
  const client = cloneClient(
    new MockServiceMethod(
      {
        clientStreaming: false,
        serverStreaming: false,
      },
      fakeCall
    )
  );

  let error: unknown;
  try {
    // Normally we would simply await `client.fakeMethod()`, but jest doesn't support
    // thenables https://github.com/jestjs/jest/issues/10501.
    await client.fakeMethod({}).then();
  } catch (e) {
    error = e;
  }

  expect(Object.getPrototypeOf(error).constructor).toEqual(Object);
  expect(error).toMatchObject(rpcErrorObjectMatcher);
});

test('response error is cloned as an object in a client streaming call', async () => {
  const send = jest.fn();
  const complete = jest.fn();
  const fakeCall: () => ClientStreamingCall = jest
    .fn()
    .mockImplementation(() => ({
      requests: {
        send,
        complete,
      },
      then: () => Promise.reject(getRpcError()),
    }));
  const client = cloneClient(
    new MockServiceMethod(
      {
        clientStreaming: true,
        serverStreaming: false,
      },
      fakeCall
    )
  );
  const res = client.fakeMethod();
  await res.requests.send({ value: 'test' });
  expect(send).toHaveBeenLastCalledWith({ value: 'test' });
  await res.requests.complete();
  expect(complete).toHaveBeenLastCalledWith();

  let error: unknown;
  try {
    await res.then();
  } catch (e) {
    error = e;
  }

  expect(Object.getPrototypeOf(error).constructor).toEqual(Object);
  expect(error).toMatchObject(rpcErrorObjectMatcher);
});

test('response error is cloned as an object in a server streaming call', async () => {
  const rejectedPromise = () => Promise.reject(getRpcError());
  const errorEmitter = new EventEmitter();
  const fakeCall: () => ServerStreamingCall = jest
    .fn()
    .mockImplementation(() => ({
      responses: {
        onNext: callback => {
          errorEmitter.on('', error => callback(undefined, error, true));
          return () => errorEmitter.off('', callback);
        },
        onError: callback => {
          errorEmitter.on('', error => callback(error));
          return () => errorEmitter.off('', callback);
        },
      } as Pick<RpcOutputStream, 'onNext' | 'onError'>,
      then: rejectedPromise,
    }));
  const client = cloneClient(
    new MockServiceMethod(
      {
        clientStreaming: false,
        serverStreaming: true,
      },
      fakeCall
    )
  );
  const res = client.fakeMethod({});
  const onNext = jest.fn();
  const onError = jest.fn();
  res.responses.onNext(onNext);
  res.responses.onError(onError);

  errorEmitter.emit('', getRpcError());
  expect(onNext).toHaveBeenCalledWith(undefined, rpcErrorObjectMatcher, true);
  expect(onError).toHaveBeenCalledWith(rpcErrorObjectMatcher);

  let error: unknown;
  try {
    await res.then();
  } catch (e) {
    error = e;
  }

  expect(Object.getPrototypeOf(error).constructor).toEqual(Object);
  expect(error).toMatchObject(rpcErrorObjectMatcher);
});

test('response error is cloned as an object in a duplex call', async () => {
  const rejectedPromise = () => Promise.reject(getRpcError());
  const errorEmitter = new EventEmitter();
  const fakeCall: () => DuplexStreamingCall = jest
    .fn()
    .mockImplementation(() => ({
      responses: {
        onNext: callback => {
          errorEmitter.on('', error => callback(undefined, error, true));
          return () => errorEmitter.off('', callback);
        },
        onError: callback => {
          errorEmitter.on('', error => callback(error));
          return () => errorEmitter.off('', callback);
        },
      } as Pick<RpcOutputStream, 'onNext' | 'onError'>,
      then: rejectedPromise,
    }));
  const client = cloneClient(
    new MockServiceMethod(
      {
        clientStreaming: true,
        serverStreaming: true,
      },
      fakeCall
    )
  );
  const res = client.fakeMethod({});
  const onNext = jest.fn();
  const onError = jest.fn();
  res.responses.onNext(onNext);
  res.responses.onError(onError);

  errorEmitter.emit('', getRpcError());
  expect(onNext).toHaveBeenCalledWith(undefined, rpcErrorObjectMatcher, true);
  expect(onError).toHaveBeenCalledWith(rpcErrorObjectMatcher);

  let error: unknown;
  try {
    await res.then();
  } catch (e) {
    error = e;
  }

  expect(Object.getPrototypeOf(error).constructor).toEqual(Object);
  expect(error).toMatchObject(rpcErrorObjectMatcher);
});

describe('isRpcError', () => {
  test.each([
    {
      name: 'returns true for plain RpcError-shaped object',
      errorToCheck: { name: 'RpcError', code: 'PERMISSION_DENIED' },
      statusCodeToCheck: 'PERMISSION_DENIED',
      expect: true,
    },
    {
      name: 'returns true for RpcError instance',
      errorToCheck: new RpcError('Denied', 'PERMISSION_DENIED'),
      statusCodeToCheck: 'PERMISSION_DENIED',
      expect: true,
    },
    {
      name: 'returns false for plain object with non-RpcError name',
      errorToCheck: { name: 'Error' },
      statusCodeToCheck: undefined,
      expect: false,
    },
    {
      name: 'returns false for Error instance',
      errorToCheck: new Error(),
      statusCodeToCheck: undefined,
      expect: false,
    },
  ])('$name', testCase => {
    expect(isRpcError(testCase.errorToCheck, testCase.statusCodeToCheck)).toBe(
      testCase.expect
    );
  });
});

describe('isRpcErrorReloginResolvable', () => {
  test.each([
    {
      name: 'returns true for plain RpcError-shaped object with relogin metadata',
      errorToCheck: {
        name: 'RpcError',
        meta: {
          'is-resolvable-with-relogin': ['1'],
        },
      },
      expect: true,
    },
    {
      name: 'returns true for RpcError instance with relogin metadata',
      errorToCheck: new RpcError('No access', 'UNKNOWN', {
        'is-resolvable-with-relogin': ['1'],
      }),
      expect: true,
    },
    {
      name: 'returns false for RpcError-shaped object without relogin metadata',
      errorToCheck: { name: 'RpcError' },
      expect: false,
    },
    {
      name: 'returns false for RpcError instance without relogin metadata',
      errorToCheck: new RpcError('No access'),
      expect: false,
    },
  ])('$name', testCase => {
    expect(isRpcErrorReloginResolvable(testCase.errorToCheck)).toBe(
      testCase.expect
    );
  });
});
