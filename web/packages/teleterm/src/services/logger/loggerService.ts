import { createLogger as createWinston, format, transports } from 'winston';
import { isObject } from 'lodash';
import { Logger, LoggerService } from './types';

export default function createLoggerService(opts: Options): LoggerService {
  const instance = createWinston({
    level: 'info',
    exitOnError: false,
    format: format.combine(
      format.timestamp({
        format: 'DD-MM-YY HH:mm:ss',
      }),
      format.printf(({ level, message, timestamp, context }) => {
        const text = stringifier(message as unknown as unknown[]);
        return `[${timestamp}] [${context}] ${level}: ${text}`;
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
    instance.add(
      new transports.Console({
        format: format.printf(({ level, message, context }) => {
          const text = stringifier(message as unknown as unknown[]);
          return `[${context}] ${level}: ${text}`;
        }),
      })
    );
  }

  return {
    createLogger(context = 'default'): Logger {
      const logger = instance.child({ context });
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

type Options = {
  dir: string;
  name: string;
  dev?: boolean;
};
