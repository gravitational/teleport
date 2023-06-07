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

import * as types from 'teleterm/types';
import { LoggerService } from 'teleterm/services/logger/types';

export default class Logger {
  private static service: types.LoggerService;
  private logger: types.Logger;

  // The Logger can be initialized in the top-level scope, but any actual
  // logging cannot be done in that scope, because we cannot guarantee that
  // Logger.init has already been called
  constructor(private context = '') {}

  warn(message: any, ...args: any[]) {
    this.getLogger().warn(message, ...args);
  }

  info(message: any, ...args: any[]) {
    this.getLogger().info(message, ...args);
  }

  error(message: any, ...args: any[]) {
    this.getLogger().error(message, ...args);
  }

  static init(service: types.LoggerService) {
    Logger.service = service;
  }

  private getLogger(): types.Logger {
    if (!this.logger) {
      if (!Logger.service) {
        throw new Error('Logger is not initialized');
      }

      this.logger = Logger.service.createLogger(this.context);
    }

    return this.logger;
  }
}

// NullService is a logger service implementation which swallows logs. For use in tests.
export class NullService implements LoggerService {
  /* eslint-disable @typescript-eslint/no-unused-vars */
  createLogger(loggerName: string): types.Logger {
    return {
      warn(...args: any[]) {},
      info(...args: any[]) {},
      error(...args: any[]) {},
    };
  }
  /* eslint-enable @typescript-eslint/no-unused-vars */
}
