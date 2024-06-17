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

import stream from 'node:stream';

import winston, {
  createLogger as createWinston,
  format,
  transports,
} from 'winston';
import { isObject } from 'shared/utils/highbar';

import split2 from 'split2';

import { Logger, LoggerService, NodeLoggerService } from './types';
import { KeepLastChunks } from './keepLastChunks';

import type { ChildProcess } from 'node:child_process';

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
        const contextLevelAndText = `${contextAndLevel} ${text}`;
        return opts.omitTimestamp
          ? contextLevelAndText
          : `[${timestamp}]${contextLevelAndText}`;
      })
    ),
    transports: [
      new transports.File({
        maxsize: 4194304, // 4 MB - max size of a single file
        maxFiles: opts.dev ? 5 : 3,
        dirname: opts.dir,
        filename: `${opts.name}.log`,
        tailable: true,
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
    pipeProcessOutputIntoLogger(
      childProcess: ChildProcess,
      lastLogs?: KeepLastChunks<string>
    ): void {
      const splitStream = split2();
      const lineToWinstonFormat = new stream.Transform({
        // Must be enabled in order for this stream to return anything else than a string or a
        // buffer from the transform function.
        objectMode: true,
        transform: (line: string, encoding, callback) => {
          callback(null, { level: 'info', message: [line] });
        },
      });

      // splitStream receives raw output from the child process and outputs lines as chunks.
      childProcess.stdout.pipe(splitStream, { end: false });
      childProcess.stderr.pipe(splitStream, { end: false });

      // lineToWinstonFormat takes each line and converts it to Winston format.
      splitStream.pipe(lineToWinstonFormat);

      // Finally, we pipe the converted lines to a Winston instance.
      lineToWinstonFormat.pipe(instance);

      // Optionally, we pipe each line to lastLogs.
      if (lastLogs) {
        splitStream.pipe(lastLogs);
      }

      // Because the .pipe calls above use { end: false }, the split stream won't end when the
      // source streams end. This gives us a chance to wait for both stdout and stderr to get closed
      // and only then end the stream.
      //
      // Otherwise the split stream would end the moment either stdout or stderr get closed, not
      // giving a chance for the other stream to pipe its data to the instance stream. This could
      // result in lost stderr logs when a process fails to start.
      childProcess.on('close', () => {
        splitStream.end();
      });
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
  Green = '42',
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
  /**
   * Does not add timestamp to log entries.
   * This has no effect on dev (console) logs, where timestamps are never added.
   * */
  omitTimestamp?: boolean;
};
