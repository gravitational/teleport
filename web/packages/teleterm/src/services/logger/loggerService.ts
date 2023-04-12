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

import winston, {
  createLogger as createWinston,
  format,
  transports,
} from 'winston';
import { isObject } from 'shared/utils/highbar';

import split2 from 'split2';

import { Logger, LoggerService, NodeLoggerService } from './types';

/**
 * stdout logger should be used in child processes.
 * It sends logs directly to stdout, so the parent logger can process that output
 * (e.g. show it in the terminal and save to a file).
 * It also allows parent to log errors emitted by the child process.
 */
export function createStdoutLoggerService(): LoggerService {
  const instance = createWinston({
    level: 'info',
    exitOnError: false,
    format: format.combine(
      format.printf(({ level, message, context }) => {
        const text = stringifier(message as unknown as unknown[]);
        return `[${context}] ${level}: ${text}`;
      })
    ),
    transports: [new transports.Console()],
  });

  return {
    createLogger(context = 'default'): Logger {
      const logger = instance.child({ context });
      return createLoggerFromWinston(logger);
    },
  };
}

/**
 * File logger saves logs directly to the file and shows them in the terminal in dev mode.
 * Can be used as a parent logger and process logs from child processes.
 */
export function createFileLoggerService(
  opts: FileLoggerOptions
): NodeLoggerService {
  const instance = createWinston({
    level: 'info',
    exitOnError: false,
    format: format.combine(
      format.timestamp({
        format: 'DD-MM-YY HH:mm:ss',
      }),
      format.printf(({ level, message, timestamp, context }) => {
        const text = stringifier(message as unknown as unknown[]);
        const contextAndLevel = opts.passThroughMode
          ? ''
          : ` [${context}] ${level}:`;
        return `[${timestamp}]${contextAndLevel} ${text}`;
      })
    ),
    transports: [
      new transports.File({
        maxsize: 4194304, // 4 MB - max size of a single file
        maxFiles: 5,
        dirname: opts.dir + '/logs',
        filename: `${opts.name}.log`,
      }),
    ],
  });

  if (opts.dev) {
    instance.add(
      new transports.Console({
        format: format.printf(({ level, message, context }) => {
          const loggerName =
            opts.loggerNameColor &&
            `\x1b[${opts.loggerNameColor}m${opts.name.toUpperCase()}\x1b[0m`;

          const text = stringifier(message as unknown as unknown[]);
          const logMessage = opts.passThroughMode
            ? text
            : `[${context}] ${level}: ${text}`;

          return [loggerName, logMessage].filter(Boolean).join(' ');
        }),
      })
    );
  }

  return {
    pipeProcessOutputIntoLogger(stream): void {
      stream
        .pipe(split2(line => ({ level: 'info', message: [line] })))
        .pipe(instance);
    },
    createLogger(context = 'default'): Logger {
      const logger = instance.child({ context });
      return createLoggerFromWinston(logger);
    },
  };
}

// maps color names to ANSI colors
export enum LoggerColor {
  Magenta = '45',
  Cyan = '46',
  Yellow = '43',
}

function createLoggerFromWinston(logger: winston.Logger): Logger {
  return {
    error: (...args) => {
      logger.error(args);
    },
    warn: (...args) => {
      logger.warn(args);
    },
    info: (...args) => {
      logger.info(args);
    },
  };
}

function stringifier(message: unknown[]): string {
  return message
    .map(singleMessage => {
      if (singleMessage instanceof Error) {
        return singleMessage.stack;
      }
      if (isObject(singleMessage)) {
        return JSON.stringify(singleMessage);
      }
      return singleMessage;
    })
    .join(' ');
}

type FileLoggerOptions = {
  dir: string;
  name: string;
  /**
   * Specifies color for the logger name e.g. SHARED, TSHD.
   * Logger name is printed in the terminal, only in dev mode.
   * If not specified, the logger name will not be printed.
   */
  loggerNameColor?: LoggerColor;
  dev?: boolean;
  /**
   * Mode for logger handling logs from other sources. Log level and context are not included in the log message.
   */
  passThroughMode?: boolean;
};
