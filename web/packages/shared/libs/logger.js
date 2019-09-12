/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*eslint no-console: "off"*/

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
