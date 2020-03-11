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

const CSS = 'color: blue';

/**
 * logger is a simple logger used by store to log changes
 */
let logger: {
  info: (...opts: any[]) => void;
  error: (...opts: any[]) => void;
  logState: (...opts: any[]) => void;
};

// init logger with the right config
if (process.env.NODE_ENV === 'development') {
  logger = {
    info(message?: string, ...optionalParams) {
      window.console.log(message, ...optionalParams);
    },

    logState(name: string, state: object, ...optionalParams) {
      window.console.log(`%cUpdated ${name} `, CSS, state, ...optionalParams);
    },

    error(err, desc) {
      if (desc) {
        window.console.error(`${desc}`, err);
      } else {
        window.console.error(err);
      }
    },
  };
} else {
  logger = {
    info() {},

    logState() {},

    error() {},
  };
}

export default logger;
