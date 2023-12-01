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
