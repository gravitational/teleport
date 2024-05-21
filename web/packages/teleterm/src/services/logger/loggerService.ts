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

import type { Logform } from 'winston';

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
        maxFiles: 5,
        dirname: opts.dir,
        filename: `${opts.name}.log`,
      }),
    ],
  });

  if (opts.dev) {
    // Browser environment.
    if (typeof window !== 'undefined') {
      instance.add(getBrowserConsoleTransport(opts));
    } else {
      instance.add(getRegularConsoleTransport(opts));
    }
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
        return JSON.stringify(
          singleMessage,
          // BigInt is not serializable with JSON.stringify
          // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/BigInt#use_within_json
          (_, value) => (typeof value === 'bigint' ? `${value}n` : value)
        );
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

/** Does not stringify messages and logs directly using `console.*` functions. */
function getBrowserConsoleTransport(opts: FileLoggerOptions) {
  return new transports.Console({
    log({ level, message, context }: Logform.TransformableInfo, next) {
      const loggerName = getLoggerName(opts);

      const logMessage = opts.passThroughMode
        ? message
        : [`[${context}] ${level}:`, ...message];

      const toLog = [loggerName, logMessage].filter(Boolean).flat();
      // We allow level to be only info, warn and error (createLoggerFromWinston).
      console[level](...toLog);
      next();
    },
  });
}

/** Stringifies log messages and logs with winston's console transport. */
function getRegularConsoleTransport(opts: FileLoggerOptions) {
  return new transports.Console({
    format: format.printf(({ level, message, context }) => {
      const loggerName = getLoggerName(opts);

      const text = stringifier(message as unknown as unknown[]);
      const logMessage = opts.passThroughMode
        ? text
        : `[${context}] ${level}: ${text}`;

      return [loggerName, logMessage].filter(Boolean).join(' ');
    }),
  });
}

function getLoggerName(
  opts: Pick<FileLoggerOptions, 'loggerNameColor' | 'name'>
) {
  return (
    opts.loggerNameColor &&
    `\x1b[${opts.loggerNameColor}m${opts.name.toUpperCase()}\x1b[0m`
  );
}
