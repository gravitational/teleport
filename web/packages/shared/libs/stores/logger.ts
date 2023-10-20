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
const isDev = process.env.NODE_ENV === 'development';

/**
 * logger is used to logs Store state changes
 */
const logger = {
  info(message?: string, ...optionalParams) {
    if (isDev) {
      window.console.log(message, ...optionalParams);
    }
  },

  logState(name: string, state: any, ...optionalParams) {
    if (isDev) {
      window.console.log(`%cUpdated ${name} `, CSS, state, ...optionalParams);
    }
  },

  error(err, desc) {
    if (!isDev) {
      return;
    }

    if (desc) {
      window.console.error(`${desc}`, err);
    } else {
      window.console.error(err);
    }
  },
};

export default logger;
