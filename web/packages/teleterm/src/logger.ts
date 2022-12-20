import * as types from 'teleterm/types';
import { LoggerService } from 'teleterm/services/logger/types';

export default class Logger {
  private static service: types.LoggerService;
  private logger: types.Logger;

  // The Logger can be initialized in the top-level scope, but any actual
  // logging cannot be done in that scope, because we cannot guarantee that
  // Logger.init has already been called
  constructor(private context = '') {}

  warn(message: string, ...args: any[]) {
    this.getLogger().warn(message, ...args);
  }

  info(message: string, ...args: any[]) {
    this.getLogger().info(message, ...args);
  }

  error(message: string, ...args: any[]) {
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
