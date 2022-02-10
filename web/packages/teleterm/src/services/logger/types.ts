export interface Logger {
  error(...args: unknown[]): void;
  warn(...args: unknown[]): void;
  info(...args: unknown[]): void;
}

export interface LoggerService {
  createLogger(context: string): Logger;
}
