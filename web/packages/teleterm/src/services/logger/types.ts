import { Stream } from 'stream';

export interface Logger {
  error(...args: unknown[]): void;
  warn(...args: unknown[]): void;
  info(...args: unknown[]): void;
}

export interface LoggerService {
  createLogger(context: string): Logger;
}

export interface NodeLoggerService extends LoggerService {
  pipeProcessOutputIntoLogger(stream: Stream): void;
}
