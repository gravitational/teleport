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

import { InterceptingCall, InterceptorOptions } from '@grpc/grpc-js';

import Logger from 'teleterm/logger';

import { withLogging } from './middleware';

it('do not log sensitive info like password', () => {
  const infoLogger = jest.fn();
  Logger.init({
    createLogger: () => ({
      info: infoLogger,
      error: () => {},
      warn: () => {},
    }),
  });
  const loggingMiddleware = withLogging(new Logger())(
    { method_definition: { path: 'LogIn' } } as InterceptorOptions,
    () =>
      ({
        sendMessageWithContext: () => {},
      } as unknown as InterceptingCall)
  );

  loggingMiddleware.sendMessage({
    toObject: () => ({
      passw: {},
      userData: {
        login: 'admin',
        password: 'admin',
      },
    }),
  });

  expect(infoLogger).toHaveBeenCalledWith(
    'send: LogIn({"passw":"~FILTERED~","userData":{"login":"admin","password":"~FILTERED~"}})'
  );
});
