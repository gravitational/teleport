import * as types from 'teleterm/types';

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
