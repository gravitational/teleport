class Logger {
  constructor(name='default') {
    this.name = name;
  }

  log(level='log', ...args) {
    console[level](`%c[${this.name}]`, `color: blue;`, ...args);
  }

  trace(...args) {
    this.log('trace', ...args);
  }

  warn(...args) {
    this.log('warn', ...args);
  }

  info(...args) {
    this.log('info', ...args);
  }

  error(...args) {
    this.log('error', ...args);
  }
}

export default {
  create: (...args) => new Logger(...args)
};
