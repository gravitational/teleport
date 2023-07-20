/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
      }) as unknown as InterceptingCall
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
