import * as types from 'teleterm/types';

export class DefaultService {
  createLogger(loggerName: string): types.Logger {
    const name = loggerName;

    const log = (level = 'log', ...args) => {
      console[level](`%c[${name}]`, `color: blue;`, ...args);
    };

    return {
      warn(...args: any[]) {
        log('warn', ...args);
      },

      info(...args: any[]) {
        log('info', ...args);
      },

      error(...args: any[]) {
        log('error', ...args);
      },
    };
  }
}

export default class Logger {
  private logger: types.Logger;

  private static service = new DefaultService();

  constructor(context = '') {
    this.logger = Logger.service.createLogger(context);
  }

  warn(message: string, ...args: any[]) {
    this.logger.warn(message, ...args);
  }

  info(message: string, ...args: any[]) {
    this.logger.info(message, ...args);
  }

  error(message: string, ...args: any[]) {
    this.logger.error(message, ...args);
  }

  static init(service: types.LoggerService) {
    Logger.service = service;
  }
}
