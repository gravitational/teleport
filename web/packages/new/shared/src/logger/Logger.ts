export class Logger {
  constructor(private name = 'default') {}

  log(
    level: 'log' | 'trace' | 'warn' | 'info' | 'debug' | 'error' = 'log',
    ...args: any[]
  ): void {
    window.console[level](`%c[${this.name}]`, `color: blue;`, ...args);
  }

  trace(...args: any[]): void {
    this.log('trace', ...args);
  }

  warn(...args: any[]): void {
    this.log('warn', ...args);
  }

  info(...args: any[]): void {
    this.log('info', ...args);
  }

  debug(...args: any[]): void {
    this.log('debug', ...args);
  }

  error(...args: any[]): void {
    this.log('error', ...args);
  }
}
