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

import { UnaryCall, MethodInfo, ServiceInfo } from '@protobuf-ts/runtime-rpc';

import Logger from 'teleterm/logger';

import { loggingInterceptor } from './interceptors';

it('do not log sensitive info like password', () => {
  const infoLogger = jest.fn();
  Logger.init({
    createLogger: () => ({
      info: infoLogger,
      error: () => {},
      warn: () => {},
    }),
  });
  const interceptor = loggingInterceptor(new Logger());

  interceptor.interceptUnary(
    () => ({ then: () => Promise.resolve({ response: '' }) }) as UnaryCall,
    {
      name: 'LogIn',
      service: { typeName: 'FooService' } as ServiceInfo,
    } as MethodInfo,
    {
      password: {},
      userData: {
        login: 'admin',
        password: 'admin',
      },
    },
    {}
  );

  expect(infoLogger).toHaveBeenCalledWith(expect.any(String), {
    password: '~FILTERED~',
    userData: { login: 'admin', password: '~FILTERED~' },
  });
});

it('includes service and method name', () => {
  const infoLogger = jest.fn();
  Logger.init({
    createLogger: () => ({
      info: infoLogger,
      error: () => {},
      warn: () => {},
    }),
  });
  const interceptor = loggingInterceptor(new Logger());

  interceptor.interceptUnary(
    () => ({ then: () => Promise.resolve({ response: '' }) }) as UnaryCall,
    {
      name: 'Foo',
      service: { typeName: 'FooService' } as ServiceInfo,
    } as MethodInfo,
    {},
    {}
  );

  expect(infoLogger).toHaveBeenCalledWith('send FooService Foo', {});
});
