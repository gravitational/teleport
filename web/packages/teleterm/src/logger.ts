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

import { LoggerService } from 'teleterm/services/logger/types';
import * as types from 'teleterm/types';

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
  /* eslint-disable unused-imports/no-unused-vars */
  createLogger(loggerName: string): types.Logger {
    return {
      warn(...args: any[]) {},
      info(...args: any[]) {},
      error(...args: any[]) {},
    };
  }
  /* eslint-enable unused-imports/no-unused-vars */
}

export class ConsoleService implements LoggerService {
  createLogger(loggerName: string): types.Logger {
    return {
      warn(...args: any[]) {
        console.warn(loggerName, ...args);
      },
      info(...args: any[]) {
        console.info(loggerName, ...args);
      },
      error(...args: any[]) {
        console.error(loggerName, ...args);
      },
    };
  }
}
