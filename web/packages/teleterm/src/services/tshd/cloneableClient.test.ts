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
  isTshdRpcError,
  TshdRpcError,
} from './cloneableClient';

function getRpcError() {
  return new RpcError('You do not have permission.', 'ACCESS_DENIED');
}

const tshdRpcErrorObjectMatcher: TshdRpcError = expect.objectContaining({
  code: 'ACCESS_DENIED',
  isResolvableWithRelogin: false,
  message: 'You do not have permission.',
  name: 'TshdRpcError',
  stack: expect.stringContaining('You do not have permission.'),
  cause: undefined,
});

class MockServiceMethod<T extends (...args: any[]) => any>
  implements ServiceInfo
{
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
  expect(error).toMatchObject(tshdRpcErrorObjectMatcher);
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
  expect(error).toMatchObject(tshdRpcErrorObjectMatcher);
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
  expect(onNext).toHaveBeenCalledWith(
    undefined,
    tshdRpcErrorObjectMatcher,
    true
  );
  expect(onError).toHaveBeenCalledWith(tshdRpcErrorObjectMatcher);

  let error: unknown;
  try {
    await res.then();
  } catch (e) {
    error = e;
  }

  expect(Object.getPrototypeOf(error).constructor).toEqual(Object);
  expect(error).toMatchObject(tshdRpcErrorObjectMatcher);
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
  expect(onNext).toHaveBeenCalledWith(
    undefined,
    tshdRpcErrorObjectMatcher,
    true
  );
  expect(onError).toHaveBeenCalledWith(tshdRpcErrorObjectMatcher);

  let error: unknown;
  try {
    await res.then();
  } catch (e) {
    error = e;
  }

  expect(Object.getPrototypeOf(error).constructor).toEqual(Object);
  expect(error).toMatchObject(tshdRpcErrorObjectMatcher);
});

test.each([
  {
    name: 'is not a tshd error',
    errorToCheck: { name: 'Error' },
    statusCodeToCheck: undefined,
    expectTshdRpcError: false,
  },
  {
    name: 'is a tshd error',
    errorToCheck: { name: 'TshdRpcError' },
    statusCodeToCheck: undefined,
    expectTshdRpcError: true,
  },
  {
    name: 'is a tshd error with a status code',
    errorToCheck: { name: 'TshdRpcError', code: 'PERMISSION_DENIED' },
    statusCodeToCheck: 'PERMISSION_DENIED',
    expectTshdRpcError: true,
  },
])('$name', testCase => {
  expect(
    isTshdRpcError(testCase.errorToCheck, testCase.statusCodeToCheck)
  ).toBe(testCase.expectTshdRpcError);
});
